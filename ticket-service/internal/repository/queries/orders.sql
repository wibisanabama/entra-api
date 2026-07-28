-- name: CreateOrder :one
INSERT INTO orders (user_id, event_id, total_amount, status, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetOrder :one
SELECT * FROM orders WHERE id = $1;

-- name: ListOrdersByUser :many
SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: GetExpiredPendingOrders :many
SELECT * FROM orders WHERE status = 'PENDING' AND expires_at < NOW();

-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, ticket_type_id, quantity, price, subtotal)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListOrderItems :many
SELECT * FROM order_items WHERE order_id = $1;
