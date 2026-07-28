package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"entra-api/payment-service/internal/repository/db"
	"entra-api/shared/kafka"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type PaymentService struct {
	queries  *db.Queries
	producer *kafka.Producer
}

func NewPaymentService(queries *db.Queries, producer *kafka.Producer) *PaymentService {
	return &PaymentService{
		queries:  queries,
		producer: producer,
	}
}

func (s *PaymentService) CreatePaymentIntent(ctx context.Context, orderID, userID string, amount float64) error {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	var amt pgtype.Numeric
	_ = amt.Scan(fmt.Sprintf("%f", amount))

	// Mock payment URL
	paymentURL := fmt.Sprintf("https://sandbox.entra.local/pay/%s", orderID)

	_, err = s.queries.CreatePayment(ctx, db.CreatePaymentParams{
		OrderID:    oid,
		UserID:     uid,
		Amount:     amt,
		Status:     "PENDING",
		PaymentUrl: pgtype.Text{String: paymentURL, Valid: true},
	})
	if err != nil {
		slog.Error("failed to create payment intent", "error", err)
		return err
	}

	return nil
}

func (s *PaymentService) SimulatePayment(ctx context.Context, paymentID string, status string) (*db.Payment, error) {
	pid, err := uuid.Parse(paymentID)
	if err != nil {
		return nil, err
	}

	payment, err := s.queries.GetPayment(ctx, pid)
	if err != nil {
		return nil, err
	}

	if payment.Status != "PENDING" {
		return nil, fmt.Errorf("payment already processed")
	}

	if status != "SUCCESS" && status != "FAILED" {
		return nil, fmt.Errorf("invalid status")
	}

	updated, err := s.queries.UpdatePaymentStatus(ctx, pid, status)
	if err != nil {
		return nil, err
	}

	// Publish event back to ticket service
	payload := map[string]interface{}{
		"order_id": payment.OrderID.String(),
		"status":   status,
	}
	payloadBytes, _ := json.Marshal(payload)

	topic := "payment.success"
	if status == "FAILED" {
		topic = "payment.failed"
	}

	_ = s.producer.Publish(ctx, topic, []byte(payment.OrderID.String()), payloadBytes)

	return &updated, nil
}

func (s *PaymentService) HandleOrderCancelled(ctx context.Context, orderID string) error {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return err
	}

	payment, err := s.queries.GetPaymentByOrderID(ctx, oid)
	if err != nil {
		return err // might not exist
	}

	if payment.Status == "PENDING" {
		_, err = s.queries.UpdatePaymentStatus(ctx, payment.ID, "EXPIRED")
		return err
	}
	return nil
}
