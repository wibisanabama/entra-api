package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Payment struct {
	ID         uuid.UUID      `json:"id"`
	OrderID    uuid.UUID      `json:"order_id"`
	UserID     uuid.UUID      `json:"user_id"`
	Amount     pgtype.Numeric `json:"amount"`
	Status     string         `json:"status"`
	PaymentUrl pgtype.Text    `json:"payment_url"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}
