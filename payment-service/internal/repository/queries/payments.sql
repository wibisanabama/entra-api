-- name: CreatePayment :one
INSERT INTO payments (reference_id, reference_type, user_id, amount, status, payment_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPayment :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByReferenceID :one
SELECT * FROM payments WHERE reference_id = $1 AND reference_type = $2;

-- name: UpdatePaymentStatus :one
UPDATE payments SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING *;
