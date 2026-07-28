package db

import (
	"context"

	"github.com/google/uuid"
)

type CreateTicketParams struct {
	OrderID      uuid.UUID `json:"order_id"`
	UserID       uuid.UUID `json:"user_id"`
	EventID      uuid.UUID `json:"event_id"`
	TicketTypeID uuid.UUID `json:"ticket_type_id"`
	TicketCode   string    `json:"ticket_code"`
}

func (q *Queries) CreateTicket(ctx context.Context, arg CreateTicketParams) (Ticket, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO tickets (order_id, user_id, event_id, ticket_type_id, ticket_code) VALUES ($1, $2, $3, $4, $5) RETURNING id, order_id, user_id, event_id, ticket_type_id, ticket_code, status, created_at, updated_at`,
		arg.OrderID, arg.UserID, arg.EventID, arg.TicketTypeID, arg.TicketCode,
	)
	var i Ticket
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.EventID, &i.TicketTypeID, &i.TicketCode, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetTicket(ctx context.Context, id uuid.UUID) (Ticket, error) {
	row := q.db.QueryRow(ctx, `SELECT id, order_id, user_id, event_id, ticket_type_id, ticket_code, status, created_at, updated_at FROM tickets WHERE id = $1`, id)
	var i Ticket
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.EventID, &i.TicketTypeID, &i.TicketCode, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetTicketByCode(ctx context.Context, ticketCode string) (Ticket, error) {
	row := q.db.QueryRow(ctx, `SELECT id, order_id, user_id, event_id, ticket_type_id, ticket_code, status, created_at, updated_at FROM tickets WHERE ticket_code = $1`, ticketCode)
	var i Ticket
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.EventID, &i.TicketTypeID, &i.TicketCode, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) ListTicketsByUser(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]Ticket, error) {
	rows, err := q.db.Query(ctx, `SELECT id, order_id, user_id, event_id, ticket_type_id, ticket_code, status, created_at, updated_at FROM tickets WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Ticket{}
	for rows.Next() {
		var i Ticket
		if err := rows.Scan(&i.ID, &i.OrderID, &i.UserID, &i.EventID, &i.TicketTypeID, &i.TicketCode, &i.Status, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) UpdateTicketStatus(ctx context.Context, id uuid.UUID, status string) (Ticket, error) {
	row := q.db.QueryRow(ctx, `UPDATE tickets SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, order_id, user_id, event_id, ticket_type_id, ticket_code, status, created_at, updated_at`, id, status)
	var i Ticket
	err := row.Scan(&i.ID, &i.OrderID, &i.UserID, &i.EventID, &i.TicketTypeID, &i.TicketCode, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}
