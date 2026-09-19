package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"entra-api/shared/kafka"
	"entra-api/ticket-service/internal/client"
	"entra-api/ticket-service/internal/repository/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var (
	ErrSoldOut                  = errors.New("tiket telah habis terjual")
	ErrActivePendingOrderExists = errors.New("anda masih memiliki pesanan yang belum diselesaikan untuk event ini")
	ErrOrderProcessing          = errors.New("pesanan Anda sedang diproses, silakan tunggu")
)

var reserveStockLua = redis.NewScript(`
local stock = tonumber(redis.call('GET', KEYS[1]))
if stock == nil then
    return -1
end
local qty = tonumber(ARGV[1])
if stock >= qty then
    redis.call('DECRBY', KEYS[1], qty)
    return 1
else
    return 0
end
`)

var releaseStockLua = redis.NewScript(`
local exists = redis.call('EXISTS', KEYS[1])
if exists == 1 then
    redis.call('INCRBY', KEYS[1], ARGV[1])
    return 1
else
    return 0
end
`)

type TicketService struct {
	pool               *pgxpool.Pool
	queries            *db.Queries
	eventClient        *client.EventClient
	producer           *kafka.Producer
	snapClient         snap.Client
	coreClient         coreapi.Client
	platformFeePercent float64
	redisClient        *redis.Client
	sf                 singleflight.Group
}

func NewTicketService(pool *pgxpool.Pool, queries *db.Queries, eventClient *client.EventClient, producer *kafka.Producer, redisClient *redis.Client) *TicketService {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		serverKey = "SB-Mid-server-dummy-key-for-dev-only" // Use placeholder if not set in env
	}

	platformFee := 5.0
	if feeEnv := os.Getenv("PLATFORM_FEE_PERCENT"); feeEnv != "" {
		if val, err := strconv.ParseFloat(feeEnv, 64); err == nil && val >= 0 {
			platformFee = val
		}
	}

	var sClient snap.Client
	sClient.New(serverKey, midtrans.Sandbox)

	var cClient coreapi.Client
	cClient.New(serverKey, midtrans.Sandbox)

	return &TicketService{
		pool:               pool,
		queries:            queries,
		eventClient:        eventClient,
		producer:           producer,
		snapClient:         sClient,
		coreClient:         cClient,
		platformFeePercent: platformFee,
		redisClient:        redisClient,
	}
}

func (s *TicketService) GetPlatformFeePercent() float64 {
	if s.platformFeePercent <= 0 {
		return 5.0
	}
	return s.platformFeePercent
}

func (s *TicketService) SetPlatformFeePercent(fee float64) {
	s.platformFeePercent = fee
}

func (s *TicketService) ReserveTicketStock(ctx context.Context, ticketTypeID string, quantity int32) error {
	if s.redisClient == nil {
		if s.eventClient != nil {
			return s.eventClient.ReserveTickets(ctx, ticketTypeID, quantity)
		}
		return nil
	}

	redisKey := fmt.Sprintf("ticket_stock:%s", ticketTypeID)
	res, err := reserveStockLua.Run(ctx, s.redisClient, []string{redisKey}, quantity).Int()
	if err == nil {
		if res == 1 {
			if s.eventClient != nil {
				_ = s.eventClient.ReserveTickets(ctx, ticketTypeID, quantity)
			}
			return nil
		}
		if res == 0 {
			return ErrSoldOut
		}
	}

	// Cache miss (-1) or Redis error: use singleflight to load stock
	v, sfErr, _ := s.sf.Do("load_stock:"+ticketTypeID, func() (interface{}, error) {
		if s.eventClient == nil {
			return int32(0), errors.New("event client not configured")
		}
		tt, getErr := s.eventClient.GetTicketType(ctx, ticketTypeID)
		if getErr != nil {
			return int32(0), getErr
		}
		avail := tt.Quantity - tt.Sold
		if avail < 0 {
			avail = 0
		}
		s.redisClient.Set(ctx, redisKey, avail, 0)
		return avail, nil
	})
	if sfErr != nil {
		if s.eventClient != nil {
			return s.eventClient.ReserveTickets(ctx, ticketTypeID, quantity)
		}
		return sfErr
	}

	availStock, _ := v.(int32)
	if availStock < quantity {
		return ErrSoldOut
	}

	retryRes, retryErr := reserveStockLua.Run(ctx, s.redisClient, []string{redisKey}, quantity).Int()
	if retryErr == nil && retryRes == 1 {
		if s.eventClient != nil {
			_ = s.eventClient.ReserveTickets(ctx, ticketTypeID, quantity)
		}
		return nil
	}

	return ErrSoldOut
}

func (s *TicketService) ReleaseTicketStock(ctx context.Context, ticketTypeID string, quantity int32) {
	if s.redisClient != nil {
		redisKey := fmt.Sprintf("ticket_stock:%s", ticketTypeID)
		_ = releaseStockLua.Run(ctx, s.redisClient, []string{redisKey}, quantity).Err()
	}
	if s.eventClient != nil {
		_ = s.eventClient.ReleaseTickets(ctx, ticketTypeID, quantity)
	}
}

type CreateOrderRequest struct {
	EventID        string  `json:"event_id" binding:"required"`
	TicketTypeID   string  `json:"ticket_type_id" binding:"required"`
	Quantity       int32   `json:"quantity" binding:"required,min=1"`
	Price          float64 `json:"price" binding:"required"`
	IdempotencyKey string  `json:"idempotency_key"`
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

	// 1. Guard against duplicate pending orders for the same user and event
	existingPending, err := s.queries.GetActivePendingOrderByUserAndEvent(ctx, db.GetActivePendingOrderByUserAndEventParams{
		UserID:  uid,
		EventID: eid,
	})
	if err == nil && existingPending.ID != uuid.Nil {
		return nil, ErrActivePendingOrderExists
	}

	// 2. Idempotency Check
	idempotencyKey := req.IdempotencyKey
	var redisIdempotencyKey string
	if idempotencyKey != "" && s.redisClient != nil {
		redisIdempotencyKey = fmt.Sprintf("order:idempotency:%s", idempotencyKey)
		cachedVal, getErr := s.redisClient.Get(ctx, redisIdempotencyKey).Result()
		if getErr == nil {
			if cachedVal == "PROCESSING" {
				return nil, ErrOrderProcessing
			}
			if orderUUID, parseErr := uuid.Parse(cachedVal); parseErr == nil {
				cachedOrder, fetchErr := s.queries.GetOrder(ctx, orderUUID)
				if fetchErr == nil {
					return &cachedOrder, nil
				}
			}
		}

		ok, setErr := s.redisClient.SetNX(ctx, redisIdempotencyKey, "PROCESSING", 5*time.Minute).Result()
		if setErr == nil && !ok {
			return nil, ErrOrderProcessing
		}
	}

	// 3. Reserve ticket stock atomically via Redis Lua
	if err := s.ReserveTicketStock(ctx, req.TicketTypeID, req.Quantity); err != nil {
		if redisIdempotencyKey != "" && s.redisClient != nil {
			s.redisClient.Del(ctx, redisIdempotencyKey)
		}
		if errors.Is(err, ErrSoldOut) {
			return nil, ErrSoldOut
		}
		slog.Error("failed to reserve tickets", "error", err)
		return nil, errors.New("failed to reserve tickets, might be sold out")
	}

	// 4. Create Order in PostgreSQL
	subtotal := float64(req.Quantity) * req.Price
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
		s.ReleaseTicketStock(ctx, req.TicketTypeID, req.Quantity)
		if redisIdempotencyKey != "" && s.redisClient != nil {
			s.redisClient.Del(ctx, redisIdempotencyKey)
		}
		return nil, err
	}

	// 5. Create Order Item
	_, err = s.queries.CreateOrderItem(ctx, db.CreateOrderItemParams{
		OrderID:      order.ID,
		TicketTypeID: tid,
		Quantity:     req.Quantity,
		Price:        totalNumeric,
		Subtotal:     totalNumeric,
	})
	if err != nil {
		slog.Error("failed to create order item", "error", err)
		_, _ = s.queries.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     order.ID,
			Status: "CANCELLED",
		})
		s.ReleaseTicketStock(ctx, req.TicketTypeID, req.Quantity)
		if redisIdempotencyKey != "" && s.redisClient != nil {
			s.redisClient.Del(ctx, redisIdempotencyKey)
		}
		return nil, errors.New("failed to initialize order items")
	}

	// 6. Cache successful order in idempotency key
	if redisIdempotencyKey != "" && s.redisClient != nil {
		s.redisClient.Set(ctx, redisIdempotencyKey, order.ID.String(), 24*time.Hour)
	}

	// 7. Publish Kafka Event
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

	if order.Status == "PAID" {
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
				"event_id":    order.EventID.String(),
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
		s.ReleaseTicketStock(ctx, item.TicketTypeID.String(), item.Quantity)
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

func (s *TicketService) CreatePaymentToken(ctx context.Context, orderID string, userID string) (string, string, error) {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return "", "", err
	}

	order, err := s.queries.GetOrder(ctx, oid)
	if err != nil {
		return "", "", err
	}

	if userID != "" && order.UserID.String() != userID {
		return "", "", errors.New("access denied: order does not belong to authenticated user")
	}

	if order.Status != "PENDING" {
		return "", "", errors.New("order is not pending")
	}

	val, err := order.TotalAmount.Float64Value()
	if err != nil {
		return "", "", err
	}
	amount := val.Float64

	// Format order_id for Midtrans (Strictly max 50 chars).
	// "ORD_" (4) + compact 32-char UUID + "_" (1) + 10-char Unix timestamp = 47 chars <= 50 limit.
	compactUUID := strings.ReplaceAll(order.ID.String(), "-", "")
	midtransOrderID := fmt.Sprintf("ORD_%s_%d", compactUUID, time.Now().Unix())

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
		if os.Getenv("APP_ENV") != "production" {
			slog.Warn("Midtrans Snap transaction creation failed in non-production, returning mock token", "error", snapErr, "order_id", orderID)
			return "MOCK_SNAP_" + midtransOrderID, midtransOrderID, nil
		}
		return "", "", snapErr
	}

	return snapResp.Token, midtransOrderID, nil
}

// SimulatePayment simulates a successful payment completion for an order (Dev / Sandbox only).
func (s *TicketService) SimulatePayment(ctx context.Context, orderID string, userID string, role string) (*db.Order, error) {
	if os.Getenv("APP_ENV") == "production" {
		return nil, errors.New("simulation is disabled in production environment")
	}

	oid, err := uuid.Parse(orderID)
	if err != nil {
		return nil, err
	}

	order, err := s.queries.GetOrder(ctx, oid)
	if err != nil {
		return nil, err
	}

	if role != "admin" && role != "organizer" && userID != "" && order.UserID.String() != userID {
		return nil, errors.New("access denied: order does not belong to authenticated user")
	}

	if order.Status == "PAID" {
		return &order, nil
	}

	if order.Status != "PENDING" {
		return nil, fmt.Errorf("cannot simulate payment for order with status %s", order.Status)
	}

	err = s.HandlePaymentSuccess(ctx, orderID)
	if err != nil {
		return nil, err
	}

	updatedOrder, err := s.queries.GetOrder(ctx, oid)
	if err != nil {
		return &order, nil
	}

	return &updatedOrder, nil
}


func (s *TicketService) HandleMidtransNotification(ctx context.Context, payload map[string]interface{}) error {
	rawOrderID, ok := payload["order_id"].(string)
	if !ok {
		return errors.New("invalid order_id in payload")
	}
	
	// Extract the real order ID (handle both ORD_<compact>_<ts> and <uuid>_<ts>)
	parts := strings.Split(rawOrderID, "_")
	var orderIDStr string
	if len(parts) >= 2 && parts[0] == "ORD" {
		orderIDStr = parts[1]
	} else {
		orderIDStr = parts[0]
	}

	orderUUID, err := uuid.Parse(orderIDStr)
	if err != nil {
		return fmt.Errorf("invalid order_id in notification %q: %w", rawOrderID, err)
	}
	orderID := orderUUID.String()
    
	txStatus, _ := payload["transaction_status"].(string)
	fraudStatus, _ := payload["fraud_status"].(string)

	tx, coreErr := s.coreClient.CheckTransaction(rawOrderID)
	if coreErr == nil && tx != nil {
		txStatus = tx.TransactionStatus
		fraudStatus = tx.FraudStatus
	} else {
		slog.Warn("midtrans CheckTransaction returned error, falling back to payload status",
			"error", coreErr,
			"raw_order_id", rawOrderID,
			"payload_status", txStatus,
		)
	}

	// If transaction_status is empty but Midtrans reported 200/201 (e.g. from frontend callback)
	if txStatus == "" {
		if statusCode, ok := payload["status_code"].(string); ok && (statusCode == "200" || statusCode == "201") {
			txStatus = "settlement"
		}
	}

	switch txStatus {
	case "capture":
		if fraudStatus == "challenge" {
			slog.Info("midtrans payment challenged", "order_id", orderID)
		} else {
			return s.HandlePaymentSuccess(ctx, orderID)
		}
	case "settlement", "success":
		return s.HandlePaymentSuccess(ctx, orderID)
	case "cancel", "deny", "expire":
		return s.HandlePaymentFailed(ctx, orderID)
	default:
		slog.Warn("unhandled or pending transaction status in midtrans notification", "status", txStatus, "order_id", orderID)
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
	TicketID        string `json:"ticket_id"`
	TicketCode      string `json:"ticket_code"`
	PreviousUserID  string `json:"previous_user_id"`
	RecipientUserID string `json:"recipient_user_id,omitempty"`
	NewOwnerEmail   string `json:"new_owner_email"`
	Status          string `json:"status"`
	TransferredAt   string `json:"transferred_at"`
	Message         string `json:"message"`
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
		return nil, errors.New("Anda bukan pemilik tiket ini")
	}

	if ticket.Status == "CHECKED_IN" || ticket.Status == "USED" {
		return nil, errors.New("tiket sudah digunakan dan tidak dapat ditransfer")
	}
	if ticket.Status == "CANCELLED" || ticket.Status == "EXPIRED" {
		return nil, errors.New("tiket sudah tidak aktif")
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(req.RecipientEmail))
	if cleanEmail == "" {
		return nil, errors.New("email penerima tidak boleh kosong")
	}

	var targetUserID uuid.UUID
	recipientName := req.RecipientName

	if req.RecipientUserID != "" {
		if uid, parseErr := uuid.Parse(req.RecipientUserID); parseErr == nil {
			targetUserID = uid
		}
	}

	// Lookup user in auth-service if recipient user_id was not directly provided
	if targetUserID == uuid.Nil {
		authServiceURL := os.Getenv("AUTH_SERVICE_URL")
		if authServiceURL == "" {
			authServiceURL = "http://localhost:8081"
		}

		endpoint := fmt.Sprintf("%s/api/v1/internal/users/by-email?email=%s", authServiceURL, url.QueryEscape(cleanEmail))
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("gagal membuat request validasi penerima: %w", err)
		}

		if secret := os.Getenv("INTERNAL_SERVICE_SECRET"); secret != "" {
			httpReq.Header.Set("X-Internal-Secret", secret)
		}

		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Do(httpReq)
		if err != nil {
			slog.ErrorContext(ctx, "failed to query auth-service for recipient lookup", "error", err, "email", cleanEmail)
			return nil, errors.New("gagal menghubungi layanan autentikasi untuk memvalidasi email penerima")
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.New("Pengguna dengan email tersebut belum terdaftar di Entra. Harap minta penerima untuk mendaftar akun terlebih dahulu.")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("layanan autentikasi mengembalikan status error: %d", resp.StatusCode)
		}

		var authRes struct {
			Data struct {
				ID       string `json:"id"`
				Email    string `json:"email"`
				FullName string `json:"full_name"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&authRes); err != nil {
			return nil, errors.New("gagal memproses respon dari layanan autentikasi")
		}

		parsedRecipientID, err := uuid.Parse(authRes.Data.ID)
		if err != nil {
			return nil, errors.New("ID pengguna penerima tidak valid")
		}
		targetUserID = parsedRecipientID

		if recipientName == "" && authRes.Data.FullName != "" {
			recipientName = authRes.Data.FullName
		}
	}

	// Prevent self-transfer
	if targetUserID == parsedSenderID {
		return nil, errors.New("Anda tidak dapat mentransfer tiket ke akun Anda sendiri")
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
			"ticket_id":         updatedTicket.ID.String(),
			"ticket_code":       updatedTicket.TicketCode,
			"event_id":          updatedTicket.EventID.String(),
			"previous_user_id":  senderUserID,
			"recipient_user_id": targetUserID.String(),
			"recipient_email":   cleanEmail,
			"recipient_name":    recipientName,
			"transferred_at":    time.Now().Format(time.RFC3339),
		})
		_ = s.producer.Publish(ctx, "ticket.transferred", []byte(updatedTicket.ID.String()), eventPayload)
	}

	return &TransferTicketResponse{
		TicketID:        updatedTicket.ID.String(),
		TicketCode:      updatedTicket.TicketCode,
		PreviousUserID:  senderUserID,
		RecipientUserID: targetUserID.String(),
		NewOwnerEmail:   cleanEmail,
		Status:          updatedTicket.Status,
		TransferredAt:   time.Now().Format(time.RFC3339),
		Message:         fmt.Sprintf("Tiket %s berhasil ditransfer ke %s", updatedTicket.TicketCode, cleanEmail),
	}, nil
}




