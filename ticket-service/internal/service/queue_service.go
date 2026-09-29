package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type QueueStatusResponse struct {
	OrderID          string    `json:"order_id"`
	EventID          string    `json:"event_id"`
	EventTitle       string    `json:"event_title,omitempty"`
	TicketTypeName   string    `json:"ticket_type_name,omitempty"`
	Quantity         int32     `json:"quantity,omitempty"`
	TotalAmount      float64   `json:"total_amount,omitempty"`
	Position         int       `json:"position"`
	Status           string    `json:"status"` // "ACTIVE", "WAITING", "COMPLETED", "CANCELLED", "EXPIRED", "SOLD_OUT"
	PeopleAhead      int       `json:"people_ahead"`
	SecondsRemaining int64     `json:"seconds_remaining"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
}

const QueueActiveDuration = 3 * time.Minute

func (s *TicketService) EnqueueOrder(ctx context.Context, orderID uuid.UUID, eventID uuid.UUID, userID uuid.UUID) (*QueueStatusResponse, error) {
	if s.redisClient == nil {
		now := time.Now()
		expiresAt := now.Add(QueueActiveDuration)
		_, _ = s.pool.Exec(ctx, "UPDATE orders SET expires_at = $2, updated_at = NOW() WHERE id = $1", orderID, expiresAt)
		return &QueueStatusResponse{
			OrderID:          orderID.String(),
			EventID:          eventID.String(),
			Position:         1,
			Status:           "ACTIVE",
			PeopleAhead:      0,
			SecondsRemaining: int64(QueueActiveDuration.Seconds()),
			ExpiresAt:        expiresAt,
		}, nil
	}

	eventKey := fmt.Sprintf("queue:event:%s", eventID.String())
	activeKey := fmt.Sprintf("%s:active", eventKey)
	expiresKey := fmt.Sprintf("%s:expires_at", eventKey)
	waitingKey := fmt.Sprintf("%s:waiting", eventKey)

	activeOrderID, err := s.redisClient.Get(ctx, activeKey).Result()
	if err == redis.Nil || activeOrderID == "" {
		// Slot is empty -> become ACTIVE immediately
		now := time.Now()
		expiresAt := now.Add(QueueActiveDuration)
		s.redisClient.Set(ctx, activeKey, orderID.String(), 24*time.Hour)
		s.redisClient.Set(ctx, expiresKey, expiresAt.Unix(), 24*time.Hour)

		_, _ = s.pool.Exec(ctx, "UPDATE orders SET expires_at = $2, updated_at = NOW() WHERE id = $1", orderID, expiresAt)

		return &QueueStatusResponse{
			OrderID:          orderID.String(),
			EventID:          eventID.String(),
			Position:         1,
			Status:           "ACTIVE",
			PeopleAhead:      0,
			SecondsRemaining: int64(QueueActiveDuration.Seconds()),
			ExpiresAt:        expiresAt,
		}, nil
	}

	// Active order exists: check if expired
	expStr, _ := s.redisClient.Get(ctx, expiresKey).Result()
	expUnix, _ := strconv.ParseInt(expStr, 10, 64)
	if expUnix > 0 && time.Now().Unix() > expUnix {
		// Existing active order has timed out, cancel it and advance
		_ = s.CancelOrder(ctx, activeOrderID)
		_, _ = s.AdvanceQueue(ctx, eventID)
		return s.EnqueueOrder(ctx, orderID, eventID, userID)
	}

	// Active order still processing -> push to waiting queue
	s.redisClient.RPush(ctx, waitingKey, orderID.String())
	llen, _ := s.redisClient.LLen(ctx, waitingKey).Result()
	pos := int(llen) + 1

	// Generous buffer in DB while waiting in line (e.g. 2 hours)
	_, _ = s.pool.Exec(ctx, "UPDATE orders SET expires_at = $2, updated_at = NOW() WHERE id = $1", orderID, time.Now().Add(2*time.Hour))

	return &QueueStatusResponse{
		OrderID:          orderID.String(),
		EventID:          eventID.String(),
		Position:         pos,
		Status:           "WAITING",
		PeopleAhead:      pos - 1,
		SecondsRemaining: 0,
	}, nil
}

func (s *TicketService) AdvanceQueue(ctx context.Context, eventID uuid.UUID) (*string, error) {
	if s.redisClient == nil {
		return nil, nil
	}

	eventKey := fmt.Sprintf("queue:event:%s", eventID.String())
	activeKey := fmt.Sprintf("%s:active", eventKey)
	expiresKey := fmt.Sprintf("%s:expires_at", eventKey)
	waitingKey := fmt.Sprintf("%s:waiting", eventKey)

	for {
		nextOrderIDStr, err := s.redisClient.LPop(ctx, waitingKey).Result()
		if err == redis.Nil || nextOrderIDStr == "" {
			s.redisClient.Del(ctx, activeKey, expiresKey)
			return nil, nil
		}

		nextOID, parseErr := uuid.Parse(nextOrderIDStr)
		if parseErr != nil {
			continue
		}

		order, fetchErr := s.queries.GetOrder(ctx, nextOID)
		if fetchErr != nil || order.Status != "PENDING" {
			// Order was already cancelled or paid, proceed to next
			continue
		}

		// Promote to ACTIVE
		now := time.Now()
		expiresAt := now.Add(QueueActiveDuration)
		s.redisClient.Set(ctx, activeKey, nextOrderIDStr, 24*time.Hour)
		s.redisClient.Set(ctx, expiresKey, expiresAt.Unix(), 24*time.Hour)
		_, _ = s.pool.Exec(ctx, "UPDATE orders SET expires_at = $2, updated_at = NOW() WHERE id = $1", nextOID, expiresAt)

		slog.Info("queue advanced: promoted next order to active", "event_id", eventID, "order_id", nextOrderIDStr, "expires_at", expiresAt)
		return &nextOrderIDStr, nil
	}
}

func (s *TicketService) CheckAndAdvanceExpiredActive(ctx context.Context, eventID uuid.UUID) {
	if s.redisClient == nil {
		return
	}

	eventKey := fmt.Sprintf("queue:event:%s", eventID.String())
	activeKey := fmt.Sprintf("%s:active", eventKey)
	expiresKey := fmt.Sprintf("%s:expires_at", eventKey)

	activeOrderID, err := s.redisClient.Get(ctx, activeKey).Result()
	if err != nil || activeOrderID == "" {
		// If no active order, see if there are waiting orders
		waitingKey := fmt.Sprintf("%s:waiting", eventKey)
		if llen, _ := s.redisClient.LLen(ctx, waitingKey).Result(); llen > 0 {
			_, _ = s.AdvanceQueue(ctx, eventID)
		}
		return
	}

	expStr, _ := s.redisClient.Get(ctx, expiresKey).Result()
	expUnix, _ := strconv.ParseInt(expStr, 10, 64)
	if expUnix > 0 && time.Now().Unix() >= expUnix {
		slog.Info("active order expired in queue, cancelling and advancing", "order_id", activeOrderID, "event_id", eventID)
		_ = s.CancelOrder(ctx, activeOrderID)
		_, _ = s.AdvanceQueue(ctx, eventID)
	}
}

func (s *TicketService) GetOrderQueueStatus(ctx context.Context, orderIDStr string, userIDStr string) (*QueueStatusResponse, error) {
	oid, err := uuid.Parse(orderIDStr)
	if err != nil {
		return nil, errors.New("invalid order id")
	}

	order, err := s.queries.GetOrder(ctx, oid)
	if err != nil {
		return nil, errors.New("order not found")
	}

	if userIDStr != "" && order.UserID.String() != userIDStr {
		return nil, errors.New("access denied")
	}

	// Prepare order summary metadata
	var totalAmt float64
	if num, valErr := order.TotalAmount.Float64Value(); valErr == nil {
		totalAmt = num.Float64
	}

	res := &QueueStatusResponse{
		OrderID:     order.ID.String(),
		EventID:     order.EventID.String(),
		TotalAmount: totalAmt,
	}

	// Fetch items for metadata
	items, itemErr := s.queries.ListOrderItems(ctx, oid)
	if itemErr == nil && len(items) > 0 {
		res.Quantity = items[0].Quantity
		if s.eventClient != nil {
			if tt, ttErr := s.eventClient.GetTicketType(ctx, items[0].TicketTypeID.String()); ttErr == nil && tt != nil {
				res.TicketTypeName = tt.Name
			}
		}
	}

	if order.Status == "PAID" || order.Status == "SUKSES" {
		res.Status = "COMPLETED"
		res.Position = 0
		return res, nil
	}

	if order.Status == "CANCELLED" {
		res.Status = "CANCELLED"
		res.Position = 0
		return res, nil
	}

	if s.redisClient == nil {
		// Fallback without redis: active with order.ExpiresAt
		rem := int64(time.Until(order.ExpiresAt).Seconds())
		if rem <= 0 {
			_ = s.CancelOrder(ctx, orderIDStr)
			res.Status = "EXPIRED"
			res.Position = 0
			return res, nil
		}
		res.Status = "ACTIVE"
		res.Position = 1
		res.SecondsRemaining = rem
		res.ExpiresAt = order.ExpiresAt
		return res, nil
	}

	// Check and advance any expired active order for this event
	s.CheckAndAdvanceExpiredActive(ctx, order.EventID)

	eventKey := fmt.Sprintf("queue:event:%s", order.EventID.String())
	activeKey := fmt.Sprintf("%s:active", eventKey)
	expiresKey := fmt.Sprintf("%s:expires_at", eventKey)
	waitingKey := fmt.Sprintf("%s:waiting", eventKey)

	// Check if this order is ACTIVE
	activeOrderID, _ := s.redisClient.Get(ctx, activeKey).Result()
	if activeOrderID == orderIDStr {
		expStr, _ := s.redisClient.Get(ctx, expiresKey).Result()
		expUnix, _ := strconv.ParseInt(expStr, 10, 64)
		rem := expUnix - time.Now().Unix()
		if rem <= 0 {
			_ = s.CancelOrder(ctx, orderIDStr)
			_, _ = s.AdvanceQueue(ctx, order.EventID)
			res.Status = "EXPIRED"
			res.Position = 0
			return res, nil
		}

		res.Status = "ACTIVE"
		res.Position = 1
		res.PeopleAhead = 0
		res.SecondsRemaining = rem
		res.ExpiresAt = time.Unix(expUnix, 0)
		return res, nil
	}

	// Check if this order is in WAITING list
	waitingList, _ := s.redisClient.LRange(ctx, waitingKey, 0, -1).Result()
	foundIdx := -1
	for i, item := range waitingList {
		if item == orderIDStr {
			foundIdx = i
			break
		}
	}

	if foundIdx >= 0 {
		pos := foundIdx + 2 // 1 is active, index 0 is position 2
		res.Status = "WAITING"
		res.Position = pos
		res.PeopleAhead = pos - 1
		res.SecondsRemaining = 0
		return res, nil
	}

	// If slot was empty or order missed, promote now
	if activeOrderID == "" {
		now := time.Now()
		expiresAt := now.Add(QueueActiveDuration)
		s.redisClient.Set(ctx, activeKey, orderIDStr, 24*time.Hour)
		s.redisClient.Set(ctx, expiresKey, expiresAt.Unix(), 24*time.Hour)
		_, _ = s.pool.Exec(ctx, "UPDATE orders SET expires_at = $2, updated_at = NOW() WHERE id = $1", oid, expiresAt)

		res.Status = "ACTIVE"
		res.Position = 1
		res.PeopleAhead = 0
		res.SecondsRemaining = int64(QueueActiveDuration.Seconds())
		res.ExpiresAt = expiresAt
		return res, nil
	}

	// If still pending, re-enqueue into waiting list
	s.redisClient.RPush(ctx, waitingKey, orderIDStr)
	llen, _ := s.redisClient.LLen(ctx, waitingKey).Result()
	pos := int(llen) + 1
	res.Status = "WAITING"
	res.Position = pos
	res.PeopleAhead = pos - 1
	return res, nil
}

func (s *TicketService) CancelOrderInQueue(ctx context.Context, orderIDStr string, userIDStr string) error {
	oid, err := uuid.Parse(orderIDStr)
	if err != nil {
		return errors.New("invalid order id")
	}

	order, err := s.queries.GetOrder(ctx, oid)
	if err != nil {
		return errors.New("order not found")
	}

	if userIDStr != "" && order.UserID.String() != userIDStr {
		return errors.New("access denied")
	}

	if order.Status == "PAID" || order.Status == "SUKSES" {
		return errors.New("tidak dapat membatalkan pesanan yang sudah dibayar")
	}

	// Cancel order and release stock
	if err := s.CancelOrder(ctx, orderIDStr); err != nil {
		return fmt.Errorf("gagal membatalkan pesanan: %w", err)
	}

	if s.redisClient != nil {
		eventKey := fmt.Sprintf("queue:event:%s", order.EventID.String())
		activeKey := fmt.Sprintf("%s:active", eventKey)
		waitingKey := fmt.Sprintf("%s:waiting", eventKey)

		activeOrderID, _ := s.redisClient.Get(ctx, activeKey).Result()
		if activeOrderID == orderIDStr {
			slog.Info("active queue order cancelled by user, advancing queue immediately", "order_id", orderIDStr, "event_id", order.EventID)
			_, _ = s.AdvanceQueue(ctx, order.EventID)
		} else {
			s.redisClient.LRem(ctx, waitingKey, 0, orderIDStr)
			slog.Info("waiting queue order cancelled by user and removed from waiting list", "order_id", orderIDStr, "event_id", order.EventID)
		}
	}

	return nil
}
