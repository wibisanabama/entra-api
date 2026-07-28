package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Order struct {
	ID          uuid.UUID      `json:"id"`
	UserID      uuid.UUID      `json:"user_id"`
	EventID     uuid.UUID      `json:"event_id"`
	TotalAmount pgtype.Numeric `json:"total_amount"`
	Status      string         `json:"status"`
	ExpiresAt   time.Time      `json:"expires_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type OrderItem struct {
	ID           uuid.UUID      `json:"id"`
	OrderID      uuid.UUID      `json:"order_id"`
	TicketTypeID uuid.UUID      `json:"ticket_type_id"`
	Quantity     int32          `json:"quantity"`
	Price        pgtype.Numeric `json:"price"`
	Subtotal     pgtype.Numeric `json:"subtotal"`
}

type Ticket struct {
	ID           uuid.UUID `json:"id"`
	OrderID      uuid.UUID `json:"order_id"`
	UserID       uuid.UUID `json:"user_id"`
	EventID      uuid.UUID `json:"event_id"`
	TicketTypeID uuid.UUID `json:"ticket_type_id"`
	TicketCode   string    `json:"ticket_code"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
