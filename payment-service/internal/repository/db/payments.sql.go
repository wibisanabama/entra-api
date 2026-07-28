package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreatePaymentParams struct {
	ReferenceID   uuid.UUID      `json:"reference_id"`
	ReferenceType string         `json:"reference_type"`
	UserID        uuid.UUID      `json:"user_id"`
	Amount        pgtype.Numeric `json:"amount"`
	Status        string         `json:"status"`
	PaymentUrl    pgtype.Text    `json:"payment_url"`
}

func (q *Queries) CreatePayment(ctx context.Context, arg CreatePaymentParams) (Payment, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO payments (reference_id, reference_type, user_id, amount, status, payment_url) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, reference_id, reference_type, user_id, amount, status, payment_url, created_at, updated_at`,
		arg.ReferenceID, arg.ReferenceType, arg.UserID, arg.Amount, arg.Status, arg.PaymentUrl,
	)
	var i Payment
	err := row.Scan(&i.ID, &i.ReferenceID, &i.ReferenceType, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetPayment(ctx context.Context, id uuid.UUID) (Payment, error) {
	row := q.db.QueryRow(ctx, `SELECT id, reference_id, reference_type, user_id, amount, status, payment_url, created_at, updated_at FROM payments WHERE id = $1`, id)
	var i Payment
	err := row.Scan(&i.ID, &i.ReferenceID, &i.ReferenceType, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

type GetPaymentByReferenceIDParams struct {
	ReferenceID   uuid.UUID `json:"reference_id"`
	ReferenceType string    `json:"reference_type"`
}

func (q *Queries) GetPaymentByReferenceID(ctx context.Context, arg GetPaymentByReferenceIDParams) (Payment, error) {
	row := q.db.QueryRow(ctx, `SELECT id, reference_id, reference_type, user_id, amount, status, payment_url, created_at, updated_at FROM payments WHERE reference_id = $1 AND reference_type = $2`, arg.ReferenceID, arg.ReferenceType)
	var i Payment
	err := row.Scan(&i.ID, &i.ReferenceID, &i.ReferenceType, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) UpdatePaymentStatus(ctx context.Context, id uuid.UUID, status string) (Payment, error) {
	row := q.db.QueryRow(ctx, `UPDATE payments SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, reference_id, reference_type, user_id, amount, status, payment_url, created_at, updated_at`, id, status)
	var i Payment
	err := row.Scan(&i.ID, &i.ReferenceID, &i.ReferenceType, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}
