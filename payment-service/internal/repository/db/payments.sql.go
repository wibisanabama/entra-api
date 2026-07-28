package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreatePaymentParams struct {
	OrderID    uuid.UUID      `json:"order_id"`
	UserID     uuid.UUID      `json:"user_id"`
	Amount     pgtype.Numeric `json:"amount"`
	Status     string         `json:"status"`
	PaymentUrl pgtype.Text    `json:"payment_url"`
}

func (q *Queries) CreatePayment(ctx context.Context, arg CreatePaymentParams) (Payment, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO payments (order_id, user_id, amount, status, payment_url) VALUES ($1, $2, $3, $4, $5) RETURNING id, order_id, user_id, amount, status, payment_url, created_at, updated_at`,
		arg.OrderID, arg.UserID, arg.Amount, arg.Status, arg.PaymentUrl,
	)
	var i Payment
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetPayment(ctx context.Context, id uuid.UUID) (Payment, error) {
	row := q.db.QueryRow(ctx, `SELECT id, order_id, user_id, amount, status, payment_url, created_at, updated_at FROM payments WHERE id = $1`, id)
	var i Payment
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetPaymentByOrderID(ctx context.Context, orderID uuid.UUID) (Payment, error) {
	row := q.db.QueryRow(ctx, `SELECT id, order_id, user_id, amount, status, payment_url, created_at, updated_at FROM payments WHERE order_id = $1`, orderID)
	var i Payment
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) UpdatePaymentStatus(ctx context.Context, id uuid.UUID, status string) (Payment, error) {
	row := q.db.QueryRow(ctx, `UPDATE payments SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, order_id, user_id, amount, status, payment_url, created_at, updated_at`, id, status)
	var i Payment
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.Amount, &i.Status, &i.PaymentUrl, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}
