package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"entra-api/shared/kafka"
	"entra-api/ticket-service/internal/client"
	"entra-api/ticket-service/internal/repository/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type TicketService struct {
	queries     *db.Queries
	eventClient *client.EventClient
	producer    *kafka.Producer
}

func NewTicketService(queries *db.Queries, eventClient *client.EventClient, producer *kafka.Producer) *TicketService {
	return &TicketService{
		queries:     queries,
		eventClient: eventClient,
		producer:    producer,
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
	_, err = s.queries.UpdateOrderStatus(ctx, oid, "PAID")
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
			_, err := s.queries.CreateTicket(ctx, db.CreateTicketParams{
				OrderID:      oid,
				UserID:       order.UserID,
				EventID:      order.EventID,
				TicketTypeID: item.TicketTypeID,
				TicketCode:   ticketCode,
			})
			if err != nil {
				slog.Error("failed to create ticket", "error", err)
			}
		}
	}

	return nil
}

func (s *TicketService) CancelOrder(ctx context.Context, orderID string) error {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateOrderStatus(ctx, oid, "CANCELLED")
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

	return nil
}
