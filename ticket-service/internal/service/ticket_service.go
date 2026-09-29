package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	ErrSoldOut                  = errors.New("Tiket telah habis terjual.")
	ErrActivePendingOrderExists = errors.New("Anda masih memiliki pesanan yang belum diselesaikan untuk event ini.")
	ErrOrderProcessing          = errors.New("Pesanan Anda sedang diproses, silakan tunggu.")
	ErrSaleEnded                = errors.New("Periode penjualan tiket untuk kategori ini telah berakhir.")
	ErrSaleNotStarted           = errors.New("Penjualan tiket untuk kategori ini belum dimulai.")
	ErrEventEnded               = errors.New("Event ini telah berakhir.")
	ErrTicketInactive           = errors.New("Kategori tiket sedang tidak aktif.")
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
		now := time.Now()
		if tt.SaleEnd != nil && now.After(*tt.SaleEnd) {
			return int32(0), ErrSaleEnded
		}
		if tt.SaleStart != nil && now.Before(*tt.SaleStart) {
			return int32(0), ErrSaleNotStarted
		}
		if tt.EventEndDate != nil && now.After(*tt.EventEndDate) {
			return int32(0), ErrEventEnded
		}
		if !tt.IsActive {
			return int32(0), ErrTicketInactive
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

	// 2.5 Check ticket validity & expiration if eventClient configured
	if s.eventClient != nil {
		tt, ttErr := s.eventClient.GetTicketType(ctx, req.TicketTypeID)
		if ttErr == nil && tt != nil {
			now := time.Now()
			if !tt.IsActive {
				if redisIdempotencyKey != "" && s.redisClient != nil {
					s.redisClient.Del(ctx, redisIdempotencyKey)
				}
				return nil, ErrTicketInactive
			}
			if tt.SaleStart != nil && now.Before(*tt.SaleStart) {
				if redisIdempotencyKey != "" && s.redisClient != nil {
					s.redisClient.Del(ctx, redisIdempotencyKey)
				}
				return nil, ErrSaleNotStarted
			}
			if tt.SaleEnd != nil && now.After(*tt.SaleEnd) {
				if redisIdempotencyKey != "" && s.redisClient != nil {
					s.redisClient.Del(ctx, redisIdempotencyKey)
				}
				return nil, ErrSaleEnded
			}
			if tt.EventEndDate != nil && now.After(*tt.EventEndDate) {
				if redisIdempotencyKey != "" && s.redisClient != nil {
					s.redisClient.Del(ctx, redisIdempotencyKey)
				}
				return nil, ErrEventEnded
			}
			if tt.EventStatus != "" && tt.EventStatus != "published" {
				if redisIdempotencyKey != "" && s.redisClient != nil {
					s.redisClient.Del(ctx, redisIdempotencyKey)
				}
				return nil, errors.New("event is not published or active")
			}
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
		if errors.Is(err, ErrSaleEnded) || errors.Is(err, ErrSaleNotStarted) || errors.Is(err, ErrEventEnded) || errors.Is(err, ErrTicketInactive) {
			return nil, err
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

	// 8. Register into Event Queue
	_, _ = s.EnqueueOrder(ctx, order.ID, eid, uid)

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

	// Advance queue for this event
	_, _ = s.AdvanceQueue(ctx, order.EventID)

	return nil
}

func (s *TicketService) CancelOrder(ctx context.Context, orderID string) error {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return err
	}

	order, err := s.queries.GetOrder(ctx, oid)
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

	// Clean up from Redis queue and advance if it was active
	if s.redisClient != nil {
		eventKey := fmt.Sprintf("queue:event:%s", order.EventID.String())
		activeKey := fmt.Sprintf("%s:active", eventKey)
		activeID, _ := s.redisClient.Get(ctx, activeKey).Result()
		if activeID == orderID {
			_, _ = s.AdvanceQueue(ctx, order.EventID)
		} else {
			s.redisClient.LRem(ctx, fmt.Sprintf("%s:waiting", eventKey), 0, orderID)
		}
	}

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

	// Verify Queue position: only active turn can request payment token
	if s.redisClient != nil {
		s.CheckAndAdvanceExpiredActive(ctx, order.EventID)
		activeKey := fmt.Sprintf("queue:event:%s:active", order.EventID.String())
		activeID := s.redisClient.Get(ctx, activeKey).Val()
		if activeID != "" && activeID != order.ID.String() {
			return "", "", errors.New("Belum giliran Anda untuk melakukan pembayaran tiket ini. Harap tunggu di antrian.")
		}
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
		return "", "", snapErr
	}

	return snapResp.Token, midtransOrderID, nil
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
