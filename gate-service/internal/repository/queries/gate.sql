-- name: CreateLocalTicket :one
INSERT INTO local_tickets (id, ticket_code, status)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetLocalTicketByCode :one
SELECT * FROM local_tickets WHERE ticket_code = $1;

-- name: UpdateLocalTicketStatus :one
UPDATE local_tickets SET status = $2 WHERE id = $1 RETURNING *;
