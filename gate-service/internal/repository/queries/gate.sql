-- name: CreateLocalTicket :one
INSERT INTO local_tickets (id, event_id, ticket_code, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLocalTicketByCode :one
SELECT * FROM local_tickets WHERE ticket_code = $1;

-- name: GetLocalTicketByID :one
SELECT * FROM local_tickets WHERE id = $1;

-- name: UpdateLocalTicketStatus :one
UPDATE local_tickets 
SET status = $2 
WHERE id = $1 AND status NOT IN ('CHECKED_IN', 'USED')
RETURNING *;

