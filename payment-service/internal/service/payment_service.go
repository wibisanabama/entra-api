package service

import (
	"context"
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
