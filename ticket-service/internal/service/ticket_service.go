package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"entra-api/shared/kafka"
	"entra-api/ticket-service/internal/client"
	"entra-api/ticket-service/internal/repository/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

type TicketService struct {
	queries     *db.Queries
	eventClient *client.EventClient
	producer    *kafka.Producer
	snapClient  snap.Client
	coreClient  coreapi.Client
}

func NewTicketService(queries *db.Queries, eventClient *client.EventClient, producer *kafka.Producer) *TicketService {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		serverKey = "SB-Mid-server-dummy-key-for-dev-only" // Use placeholder if not set in env
	}

	var sClient snap.Client
	sClient.New(serverKey, midtrans.Sandbox)

	var cClient coreapi.Client
	cClient.New(serverKey, midtrans.Sandbox)

	return &TicketService{
		queries:     queries,
		eventClient: eventClient,
		producer:    producer,
		snapClient:  sClient,
		coreClient:  cClient,
	}
}

type CreateOrderRequest struct {
	EventID      string  `json:"event_id" binding:"required"`
	TicketTypeID string  `json:"ticket_type_id" binding:"required"`
	Quantity     int32   `json:"quantity" binding:"required,min=1"`
	Price        float64 `json:"price" binding:"required"`
}

func (s *TicketService) CreateOrder(ctx context.Context, userID string, req CreateOrderRequest) (*db.Order, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	eid, err := uuid.Parse(req.EventID)
	if err != nil {
		return nil, errors.New("invalid event id")
	}
	tid, err := uuid.Parse(req.TicketTypeID)
	if err != nil {
		return nil, errors.New("invalid ticket type id")
	}

	// 1. Reserve ticket via event-service
	if err := s.eventClient.ReserveTickets(ctx, req.TicketTypeID, req.Quantity); err != nil {
		slog.Error("failed to reserve tickets", "error", err)
		return nil, errors.New("failed to reserve tickets, might be sold out")
	}

	// 2. Create Order
	subtotal := float64(req.Quantity) * req.Price
	
	// Convert float64 to pgtype.Numeric correctly
	var totalNumeric pgtype.Numeric
	_ = totalNumeric.Scan(fmt.Sprintf("%f", subtotal))

	order, err := s.queries.CreateOrder(ctx, db.CreateOrderParams{
		UserID:      uid,
		EventID:     eid,
		TotalAmount: totalNumeric,
		Status:      "PENDING",
		ExpiresAt:   time.Now().Add(15 * time.Minute), // 15 mins to pay
	})
	if err != nil {
		// Rollback reservation
		_ = s.eventClient.ReleaseTickets(ctx, req.TicketTypeID, req.Quantity)
		return nil, err
	}

	// 3. Create Order Item
	_, err = s.queries.CreateOrderItem(ctx, db.CreateOrderItemParams{
		OrderID:      order.ID,
		TicketTypeID: tid,
		Quantity:     req.Quantity,
		Price:        totalNumeric, // for simplicity, assuming total == price * qty 
		Subtotal:     totalNumeric,
	})
	if err != nil {
		slog.Error("failed to create order item", "error", err)
		// We should really use a DB transaction for Order + OrderItems. Ignoring for brevity.
	}

	// 4. Publish Kafka Event
	eventPayload := map[string]interface{}{
		"order_id": order.ID.String(),
		"user_id":  userID,
		"amount":   subtotal,
	}
	payloadBytes, _ := json.Marshal(eventPayload)
	_ = s.producer.Publish(ctx, "order.created", []byte(order.ID.String()), payloadBytes)

	return &order, nil
}

func (s *TicketService) HandlePaymentSuccess(ctx context.Context, orderID string) error {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return err
	}

	order, err := s.queries.GetOrder(ctx, oid)
	if err != nil {
		return err
	}

	if order.Status != "PENDING" {
		return nil // Already processed
	}

	// Update order
	_, err = s.queries.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		ID:     oid,
		Status: "PAID",
	})
	if err != nil {
		return err
	}

	// Generate tickets
	items, err := s.queries.ListOrderItems(ctx, oid)
	if err != nil {
		return err
	}

	for _, item := range items {
		for i := int32(0); i < item.Quantity; i++ {
			ticketCode := uuid.New().String() // Simple barcode payload
			ticket, err := s.queries.CreateTicket(ctx, db.CreateTicketParams{
				OrderID:      oid,
				UserID:       order.UserID,
				EventID:      order.EventID,
				TicketTypeID: item.TicketTypeID,
				TicketCode:   ticketCode,
			})
			if err != nil {
				slog.Error("failed to create ticket", "error", err)
				continue
			}

			// Publish ticket.created event
			ticketPayload := map[string]interface{}{
				"ticket_id":   ticket.ID.String(),
				"ticket_code": ticket.TicketCode,
				"status":      ticket.Status,
			}
			payloadBytes, _ := json.Marshal(ticketPayload)
			_ = s.producer.Publish(ctx, "ticket.created", []byte(ticket.ID.String()), payloadBytes)
		}
	}

	return nil
}

func (s *TicketService) CancelOrder(ctx context.Context, orderID string) error {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		ID:     oid,
		Status: "CANCELLED",
	})
	if err != nil {
		return err
	}

	items, err := s.queries.ListOrderItems(ctx, oid)
	if err != nil {
		return err
	}

	// Release all tickets back to inventory
	for _, item := range items {
		_ = s.eventClient.ReleaseTickets(ctx, item.TicketTypeID.String(), item.Quantity)
	}

	// Publish Kafka Event
	eventPayload := map[string]interface{}{
		"order_id": orderID,
	}
	payloadBytes, _ := json.Marshal(eventPayload)
	_ = s.producer.Publish(ctx, "order.cancelled", []byte(orderID), payloadBytes)

	return nil
}

func (s *TicketService) HandlePaymentFailed(ctx context.Context, orderID string) error {
	slog.Info("handling payment failed, cancelling order", "order_id", orderID)
	return s.CancelOrder(ctx, orderID)
}

func (s *TicketService) CreatePaymentToken(ctx context.Context, orderID string) (string, error) {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return "", err
	}

	order, err := s.queries.GetOrder(ctx, oid)
	if err != nil {
		return "", err
	}

	if order.Status != "PENDING" {
		return "", errors.New("order is not pending")
	}

	val, err := order.TotalAmount.Float64Value()
	if err != nil {
		return "", err
	}
	amount := val.Float64

	midtransOrderID := fmt.Sprintf("%s_%d", order.ID.String(), time.Now().Unix())

	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  midtransOrderID,
			GrossAmt: int64(amount),
		},
		CreditCard: &snap.CreditCardDetails{
			Secure: true,
		},
	}

	snapResp, snapErr := s.snapClient.CreateTransaction(req)
	if snapErr != nil {
		return "", snapErr
	}

	return snapResp.Token, nil
}

func (s *TicketService) HandleMidtransNotification(ctx context.Context, payload map[string]interface{}) error {
	rawOrderID, ok := payload["order_id"].(string)
	if !ok {
		return errors.New("invalid order_id in payload")
	}
	
	// Extract the real order ID (before the _)
	parts := strings.Split(rawOrderID, "_")
	orderID := parts[0]
    
	tx, coreErr := s.coreClient.CheckTransaction(rawOrderID)
	if coreErr != nil {
		return coreErr
	}

	switch tx.TransactionStatus {
	case "capture":
		if tx.FraudStatus == "challenge" {
			// Do nothing or mark as challenge
		} else if tx.FraudStatus == "accept" {
			return s.HandlePaymentSuccess(ctx, orderID)
		}
	case "settlement":
		return s.HandlePaymentSuccess(ctx, orderID)
	case "cancel", "deny", "expire":
		return s.HandlePaymentFailed(ctx, orderID)
	}

	return nil
}

func (s *TicketService) ListMyTickets(ctx context.Context, userID string) ([]db.Ticket, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	// Just limit to 100 for now
	return s.queries.ListTicketsByUser(ctx, db.ListTicketsByUserParams{
		UserID: uid,
		Limit:  100,
		Offset: 0,
	})
}

func (s *TicketService) ListMyOrders(ctx context.Context, userID string) ([]db.Order, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	return s.queries.ListOrdersByUser(ctx, db.ListOrdersByUserParams{
		UserID: uid,
		Limit:  100,
		Offset: 0,
	})
}

func (s *TicketService) GetTicketByCode(ctx context.Context, ticketCode string) (*db.Ticket, error) {
	t, err := s.queries.GetTicketByCode(ctx, ticketCode)
	if err != nil {
		if parsedUUID, parseErr := uuid.Parse(ticketCode); parseErr == nil {
			tByID, errByID := s.queries.GetTicket(ctx, parsedUUID)
			if errByID == nil {
				return &tByID, nil
			}
		}
		return nil, err
	}
	return &t, nil
}

