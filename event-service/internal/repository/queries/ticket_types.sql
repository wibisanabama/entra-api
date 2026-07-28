-- name: CreateTicketType :one
INSERT INTO ticket_types (
    event_id, name, description, price, quantity, max_per_order,
    sale_start, sale_end
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTicketTypeByID :one
SELECT * FROM ticket_types WHERE id = $1;

-- name: ListTicketTypesByEvent :many
SELECT * FROM ticket_types
WHERE event_id = $1 AND is_active = TRUE
ORDER BY price ASC;

-- name: UpdateTicketType :one
UPDATE ticket_types SET
    name = $2, description = $3, price = $4, quantity = $5,
    max_per_order = $6, sale_start = $7, sale_end = $8, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteTicketType :exec
DELETE FROM ticket_types WHERE id = $1;

-- name: IncrementTicketSold :one
UPDATE ticket_types SET sold = sold + $2, updated_at = NOW()
WHERE id = $1 AND (quantity - sold) >= $2
RETURNING *;

-- name: DecrementTicketSold :one
UPDATE ticket_types SET sold = sold - $2, updated_at = NOW()
WHERE id = $1 AND sold >= $2
RETURNING *;
