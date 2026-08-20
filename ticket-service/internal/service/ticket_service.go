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
		_, _ = s.queries.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     order.ID,
			Status: "CANCELLED",
		})
		_ = s.eventClient.ReleaseTickets(ctx, req.TicketTypeID, req.Quantity)
		return nil, errors.New("failed to initialize order items")
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

type EventGateStats struct {
	EventID      string  `json:"event_id"`
	TotalTickets int     `json:"total_tickets"`
	CheckedIn    int     `json:"checked_in"`
	Remaining    int     `json:"remaining"`
	CheckInRate  float64 `json:"checkin_rate"`
	Status       string  `json:"status"`
}

func (s *TicketService) GetEventGateStats(ctx context.Context, eventID string) (*EventGateStats, error) {
	uid, err := uuid.Parse(eventID)
	if err != nil {
		return nil, fmt.Errorf("invalid event_id: %w", err)
	}

	tickets, err := s.queries.ListTicketsByEvent(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets for event: %w", err)
	}

	total := len(tickets)
	checkedIn := 0
	for _, t := range tickets {
		if t.Status == "CHECKED_IN" || t.Status == "USED" {
			checkedIn++
		}
	}

	remaining := total - checkedIn
	rate := 0.0
	if total > 0 {
		rate = float64(checkedIn) / float64(total) * 100.0
	}

	status := "NOT_STARTED"
	if checkedIn > 0 && checkedIn < total {
		status = "IN_PROGRESS"
	} else if checkedIn > 0 && checkedIn == total {
		status = "COMPLETED"
	}

	return &EventGateStats{
		EventID:      eventID,
		TotalTickets: total,
		CheckedIn:    checkedIn,
		Remaining:    remaining,
		CheckInRate:  rate,
		Status:       status,
	}, nil
}

type ValidatePromoRequest struct {
	PromoCode      string  `json:"promo_code" binding:"required"`
	Subtotal       float64 `json:"subtotal" binding:"required,min=0"`
	EventID        string  `json:"event_id"`
	TicketQuantity int     `json:"ticket_quantity"`
}

type ValidatePromoResponse struct {
	IsValid        bool    `json:"is_valid"`
	PromoCode      string  `json:"promo_code"`
	DiscountType   string  `json:"discount_type"` // "PERCENTAGE" | "FLAT"
	DiscountValue  float64 `json:"discount_value"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalTotal     float64 `json:"final_total"`
	Message        string  `json:"message"`
}

func (s *TicketService) ValidatePromo(ctx context.Context, req ValidatePromoRequest) (*ValidatePromoResponse, error) {
	code := strings.ToUpper(strings.TrimSpace(req.PromoCode))
	if code == "" {
		return nil, errors.New("kode promo tidak boleh kosong")
	}

	var discountType string
	var discountValue float64
	var maxDiscount float64 = 0
	var minOrder float64 = 0
	var minQty int = 0
	var desc string

	switch code {
	case "ENTRA20", "ENTRAPROMO":
		discountType = "PERCENTAGE"
		discountValue = 20.0
		maxDiscount = 100000.0
		minOrder = 50000.0
		desc = "Diskon 20% (Maks Rp 100.000)"
	case "FESTIVAL50", "SUPERDEAL":
		discountType = "FLAT"
		discountValue = 50000.0
		minOrder = 150000.0
		desc = "Potongan Langsung Rp 50.000"
	case "WELCOME10", "NEWUSER":
		discountType = "PERCENTAGE"
		discountValue = 10.0
		maxDiscount = 50000.0
		minOrder = 0.0
		desc = "Diskon Pengguna Baru 10% (Maks Rp 50.000)"
	case "VIPPASS", "SPECIALPASS":
		discountType = "PERCENTAGE"
		discountValue = 15.0
		maxDiscount = 150000.0
		minQty = 2
		desc = "Diskon Spesial Rombongan 15% (Min 2 Tiket)"
	case "FLASHDEAL":
		discountType = "FLAT"
		discountValue = 25000.0
		minOrder = 75000.0
		desc = "Potongan Flash Deal Rp 25.000"
	default:
		return &ValidatePromoResponse{
			IsValid:   false,
			PromoCode: code,
			Message:   "Kode promo tidak valid atau sudah kedaluwarsa.",
		}, nil
	}

	if req.Subtotal < minOrder {
		return &ValidatePromoResponse{
			IsValid:   false,
			PromoCode: code,
			Message:   fmt.Sprintf("Kode promo %s membutuhkan minimal transaksi Rp %.0f.", code, minOrder),
		}, nil
	}

	if req.TicketQuantity > 0 && req.TicketQuantity < minQty {
		return &ValidatePromoResponse{
			IsValid:   false,
			PromoCode: code,
			Message:   fmt.Sprintf("Kode promo %s membutuhkan minimal pembelian %d tiket.", code, minQty),
		}, nil
	}

	var discountAmount float64
	if discountType == "PERCENTAGE" {
		discountAmount = (discountValue / 100.0) * req.Subtotal
		if maxDiscount > 0 && discountAmount > maxDiscount {
			discountAmount = maxDiscount
		}
	} else {
		discountAmount = discountValue
		if discountAmount > req.Subtotal {
			discountAmount = req.Subtotal
		}
	}

	finalTotal := req.Subtotal - discountAmount
	if finalTotal < 0 {
		finalTotal = 0
	}

	return &ValidatePromoResponse{
		IsValid:        true,
		PromoCode:      code,
		DiscountType:   discountType,
		DiscountValue:  discountValue,
		DiscountAmount: discountAmount,
		FinalTotal:     finalTotal,
		Message:        fmt.Sprintf("Kupon promo %s berhasil diterapkan! %s", code, desc),
	}, nil
}

type TransferTicketRequest struct {
	RecipientEmail  string `json:"recipient_email" binding:"required,email"`
	RecipientName   string `json:"recipient_name"`
	RecipientUserID string `json:"recipient_user_id"`
	Reason          string `json:"reason"`
}

type TransferTicketResponse struct {
	TicketID       string `json:"ticket_id"`
	TicketCode     string `json:"ticket_code"`
	PreviousUserID string `json:"previous_user_id"`
	NewOwnerEmail  string `json:"new_owner_email"`
	Status         string `json:"status"`
	TransferredAt  string `json:"transferred_at"`
	Message        string `json:"message"`
}

func (s *TicketService) TransferTicket(ctx context.Context, senderUserID string, ticketID string, req TransferTicketRequest) (*TransferTicketResponse, error) {
	parsedSenderID, err := uuid.Parse(senderUserID)
	if err != nil {
		return nil, errors.New("invalid sender user_id")
	}

	parsedTicketID, err := uuid.Parse(ticketID)
	if err != nil {
		return nil, errors.New("invalid ticket_id")
	}

	ticket, err := s.queries.GetTicket(ctx, parsedTicketID)
	if err != nil {
		return nil, errors.New("tiket tidak ditemukan")
	}

	if ticket.UserID != parsedSenderID {
		return nil, errors.New("anda bukan pemilik tiket ini")
	}

	if ticket.Status == "CHECKED_IN" || ticket.Status == "USED" {
		return nil, errors.New("tiket sudah digunakan dan tidak dapat ditransfer")
	}
	if ticket.Status == "CANCELLED" || ticket.Status == "EXPIRED" {
		return nil, errors.New("tiket sudah tidak aktif")
	}

	var targetUserID uuid.UUID
	if req.RecipientUserID != "" {
		if uid, parseErr := uuid.Parse(req.RecipientUserID); parseErr == nil {
			targetUserID = uid
		}
	}
	if targetUserID == uuid.Nil {
		targetUserID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.ToLower(strings.TrimSpace(req.RecipientEmail))))
	}

	updatedTicket, err := s.queries.UpdateTicketOwner(ctx, db.UpdateTicketOwnerParams{
		ID:     ticket.ID,
		UserID: targetUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("gagal memindahkan kepemilikan tiket: %w", err)
	}

	if s.producer != nil {
		eventPayload, _ := json.Marshal(map[string]interface{}{
			"ticket_id":        updatedTicket.ID.String(),
			"ticket_code":      updatedTicket.TicketCode,
			"event_id":         updatedTicket.EventID.String(),
			"previous_user_id": senderUserID,
			"recipient_email":  req.RecipientEmail,
			"recipient_name":   req.RecipientName,
			"transferred_at":   time.Now().Format(time.RFC3339),
		})
		_ = s.producer.Publish(ctx, "ticket.transferred", []byte(updatedTicket.ID.String()), eventPayload)
	}

	return &TransferTicketResponse{
		TicketID:       updatedTicket.ID.String(),
		TicketCode:     updatedTicket.TicketCode,
		PreviousUserID: senderUserID,
		NewOwnerEmail:  req.RecipientEmail,
		Status:         updatedTicket.Status,
		TransferredAt:  time.Now().Format(time.RFC3339),
		Message:        fmt.Sprintf("Tiket %s berhasil ditransfer ke %s", updatedTicket.TicketCode, req.RecipientEmail),
	}, nil
}




