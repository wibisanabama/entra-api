-- name: CreateTicket :one
INSERT INTO tickets (order_id, user_id, event_id, ticket_type_id, ticket_code)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTicket :one
SELECT * FROM tickets WHERE id = $1;

-- name: GetTicketByCode :one
SELECT * FROM tickets WHERE ticket_code = $1;

-- name: ListTicketsByUser :many
SELECT * FROM tickets WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: UpdateTicketStatus :one
UPDATE tickets SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: ListTicketsByEvent :many
SELECT * FROM tickets WHERE event_id = $1 ORDER BY created_at ASC;

-- name: ListTicketsByOrder :many
SELECT * FROM tickets WHERE order_id = $1 ORDER BY created_at ASC;
