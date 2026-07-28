-- name: CreatePayment :one
INSERT INTO payments (order_id, user_id, amount, status, payment_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPayment :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByOrderID :one
SELECT * FROM payments WHERE order_id = $1;

-- name: UpdatePaymentStatus :one
UPDATE payments SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING *;
