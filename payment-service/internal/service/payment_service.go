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

func (s *PaymentService) CreatePaymentIntent(ctx context.Context, referenceID, referenceType, userID string, amount float64) error {
	rid, err := uuid.Parse(referenceID)
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
	paymentURL := fmt.Sprintf("https://sandbox.entra.local/pay/%s", referenceID)

	_, err = s.queries.CreatePayment(ctx, db.CreatePaymentParams{
		ReferenceID:   rid,
		ReferenceType: referenceType,
		UserID:        uid,
		Amount:        amt,
		Status:        "PENDING",
		PaymentUrl:    pgtype.Text{String: paymentURL, Valid: true},
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

	// Publish event back to originating service
	payload := map[string]interface{}{
		"reference_id":   payment.ReferenceID.String(),
		"reference_type": payment.ReferenceType,
		"status":         status,
	}
	// Also populate order_id for backward compatibility with ticket-service, or just refactor ticket service to read reference_id if needed.
	// But it's cleaner if ticket service expects reference_id and reference_type now. We'll update ticket service.
	payloadBytes, _ := json.Marshal(payload)

	topic := "payment.success"
	if status == "FAILED" {
		topic = "payment.failed"
	}

	_ = s.producer.Publish(ctx, topic, []byte(payment.ReferenceID.String()), payloadBytes)

	return &updated, nil
}

func (s *PaymentService) HandleReferenceCancelled(ctx context.Context, referenceID, referenceType string) error {
	rid, err := uuid.Parse(referenceID)
	if err != nil {
		return err
	}

	payment, err := s.queries.GetPaymentByReferenceID(ctx, db.GetPaymentByReferenceIDParams{
		ReferenceID:   rid,
		ReferenceType: referenceType,
	})
	if err != nil {
		return err // might not exist
	}

	if payment.Status == "PENDING" {
		_, err = s.queries.UpdatePaymentStatus(ctx, payment.ID, "EXPIRED")
		return err
	}
	return nil
}
