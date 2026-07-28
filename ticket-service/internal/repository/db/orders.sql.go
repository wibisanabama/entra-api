package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateOrderItemParams struct {
	OrderID      uuid.UUID      `json:"order_id"`
	TicketTypeID uuid.UUID      `json:"ticket_type_id"`
	Quantity     int32          `json:"quantity"`
	Price        pgtype.Numeric `json:"price"`
	Subtotal     pgtype.Numeric `json:"subtotal"`
}

func (q *Queries) CreateOrderItem(ctx context.Context, arg CreateOrderItemParams) (OrderItem, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO order_items (order_id, ticket_type_id, quantity, price, subtotal) VALUES ($1, $2, $3, $4, $5) RETURNING id, order_id, ticket_type_id, quantity, price, subtotal`,
		arg.OrderID, arg.TicketTypeID, arg.Quantity, arg.Price, arg.Subtotal,
	)
	var i OrderItem
	err := row.Scan(&i.ID, &i.OrderID, &i.TicketTypeID, &i.Quantity, &i.Price, &i.Subtotal)
	return i, err
}

type CreateOrderParams struct {
	UserID      uuid.UUID      `json:"user_id"`
	EventID     uuid.UUID      `json:"event_id"`
	TotalAmount pgtype.Numeric `json:"total_amount"`
	Status      string         `json:"status"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

func (q *Queries) CreateOrder(ctx context.Context, arg CreateOrderParams) (Order, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO orders (user_id, event_id, total_amount, status, expires_at) VALUES ($1, $2, $3, $4, $5) RETURNING id, user_id, event_id, total_amount, status, expires_at, created_at, updated_at`,
		arg.UserID, arg.EventID, arg.TotalAmount, arg.Status, arg.ExpiresAt,
	)
	var i Order
	err := row.Scan(&i.ID, &i.UserID, &i.EventID, &i.TotalAmount, &i.Status, &i.ExpiresAt, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetExpiredPendingOrders(ctx context.Context) ([]Order, error) {
	rows, err := q.db.Query(ctx, `SELECT id, user_id, event_id, total_amount, status, expires_at, created_at, updated_at FROM orders WHERE status = 'PENDING' AND expires_at < NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Order{}
	for rows.Next() {
		var i Order
		if err := rows.Scan(&i.ID, &i.UserID, &i.EventID, &i.TotalAmount, &i.Status, &i.ExpiresAt, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) GetOrder(ctx context.Context, id uuid.UUID) (Order, error) {
	row := q.db.QueryRow(ctx, `SELECT id, user_id, event_id, total_amount, status, expires_at, created_at, updated_at FROM orders WHERE id = $1`, id)
	var i Order
	err := row.Scan(&i.ID, &i.UserID, &i.EventID, &i.TotalAmount, &i.Status, &i.ExpiresAt, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) ListOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error) {
	rows, err := q.db.Query(ctx, `SELECT id, order_id, ticket_type_id, quantity, price, subtotal FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []OrderItem{}
	for rows.Next() {
		var i OrderItem
		if err := rows.Scan(&i.ID, &i.OrderID, &i.TicketTypeID, &i.Quantity, &i.Price, &i.Subtotal); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) ListOrdersByUser(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]Order, error) {
	rows, err := q.db.Query(ctx, `SELECT id, user_id, event_id, total_amount, status, expires_at, created_at, updated_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Order{}
	for rows.Next() {
		var i Order
		if err := rows.Scan(&i.ID, &i.UserID, &i.EventID, &i.TotalAmount, &i.Status, &i.ExpiresAt, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) (Order, error) {
	row := q.db.QueryRow(ctx, `UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, user_id, event_id, total_amount, status, expires_at, created_at, updated_at`, id, status)
	var i Order
	err := row.Scan(&i.ID, &i.UserID, &i.EventID, &i.TotalAmount, &i.Status, &i.ExpiresAt, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}
