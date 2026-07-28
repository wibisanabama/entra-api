package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateTicketTypeParams struct {
	EventID     uuid.UUID          `json:"event_id"`
	Name        string             `json:"name"`
	Description pgtype.Text        `json:"description"`
	Price       pgtype.Numeric     `json:"price"`
	Quantity    int32              `json:"quantity"`
	MaxPerOrder pgtype.Int4        `json:"max_per_order"`
	SaleStart   pgtype.Timestamptz `json:"sale_start"`
	SaleEnd     pgtype.Timestamptz `json:"sale_end"`
}

func (q *Queries) CreateTicketType(ctx context.Context, arg CreateTicketTypeParams) (TicketType, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO ticket_types (event_id, name, description, price, quantity, max_per_order, sale_start, sale_end) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, event_id, name, description, price, quantity, sold, max_per_order, sale_start, sale_end, is_active, created_at, updated_at`,
		arg.EventID, arg.Name, arg.Description, arg.Price, arg.Quantity, arg.MaxPerOrder, arg.SaleStart, arg.SaleEnd,
	)
	var i TicketType
	err := row.Scan(&i.ID, &i.EventID, &i.Name, &i.Description, &i.Price, &i.Quantity, &i.Sold, &i.MaxPerOrder, &i.SaleStart, &i.SaleEnd, &i.IsActive, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetTicketTypeByID(ctx context.Context, id uuid.UUID) (TicketType, error) {
	row := q.db.QueryRow(ctx, `SELECT id, event_id, name, description, price, quantity, sold, max_per_order, sale_start, sale_end, is_active, created_at, updated_at FROM ticket_types WHERE id = $1`, id)
	var i TicketType
	err := row.Scan(&i.ID, &i.EventID, &i.Name, &i.Description, &i.Price, &i.Quantity, &i.Sold, &i.MaxPerOrder, &i.SaleStart, &i.SaleEnd, &i.IsActive, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) ListTicketTypesByEvent(ctx context.Context, eventID uuid.UUID) ([]TicketType, error) {
	rows, err := q.db.Query(ctx, `SELECT id, event_id, name, description, price, quantity, sold, max_per_order, sale_start, sale_end, is_active, created_at, updated_at FROM ticket_types WHERE event_id = $1 AND is_active = TRUE ORDER BY price ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TicketType{}
	for rows.Next() {
		var i TicketType
		if err := rows.Scan(&i.ID, &i.EventID, &i.Name, &i.Description, &i.Price, &i.Quantity, &i.Sold, &i.MaxPerOrder, &i.SaleStart, &i.SaleEnd, &i.IsActive, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type UpdateTicketTypeParams struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Description pgtype.Text        `json:"description"`
	Price       pgtype.Numeric     `json:"price"`
	Quantity    int32              `json:"quantity"`
	MaxPerOrder pgtype.Int4        `json:"max_per_order"`
	SaleStart   pgtype.Timestamptz `json:"sale_start"`
	SaleEnd     pgtype.Timestamptz `json:"sale_end"`
}

func (q *Queries) UpdateTicketType(ctx context.Context, arg UpdateTicketTypeParams) (TicketType, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE ticket_types SET name = $2, description = $3, price = $4, quantity = $5, max_per_order = $6, sale_start = $7, sale_end = $8, updated_at = NOW() WHERE id = $1 RETURNING id, event_id, name, description, price, quantity, sold, max_per_order, sale_start, sale_end, is_active, created_at, updated_at`,
		arg.ID, arg.Name, arg.Description, arg.Price, arg.Quantity, arg.MaxPerOrder, arg.SaleStart, arg.SaleEnd,
	)
	var i TicketType
	err := row.Scan(&i.ID, &i.EventID, &i.Name, &i.Description, &i.Price, &i.Quantity, &i.Sold, &i.MaxPerOrder, &i.SaleStart, &i.SaleEnd, &i.IsActive, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) DeleteTicketType(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, `DELETE FROM ticket_types WHERE id = $1`, id)
	return err
}

func (q *Queries) IncrementTicketSold(ctx context.Context, id uuid.UUID, amount int32) (TicketType, error) {
	row := q.db.QueryRow(ctx, `UPDATE ticket_types SET sold = sold + $2, updated_at = NOW() WHERE id = $1 AND (quantity - sold) >= $2 RETURNING id, event_id, name, description, price, quantity, sold, max_per_order, sale_start, sale_end, is_active, created_at, updated_at`, id, amount)
	var i TicketType
	err := row.Scan(&i.ID, &i.EventID, &i.Name, &i.Description, &i.Price, &i.Quantity, &i.Sold, &i.MaxPerOrder, &i.SaleStart, &i.SaleEnd, &i.IsActive, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}
