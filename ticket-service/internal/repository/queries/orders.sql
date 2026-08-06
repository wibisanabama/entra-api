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

-- name: GetOrganizerStats :one
SELECT 
    COALESCE(COUNT(DISTINCT id), 0)::bigint as total_orders,
    COALESCE(SUM(total_amount), 0)::numeric as total_revenue,
    COALESCE(SUM((SELECT SUM(quantity) FROM order_items WHERE order_items.order_id = orders.id)), 0)::bigint as tickets_sold
FROM orders 
WHERE event_id = ANY($1::uuid[]) AND status = 'SUKSES';

-- name: ListOrdersByEvents :many
SELECT * FROM orders 
WHERE event_id = ANY($1::uuid[])
ORDER BY created_at DESC 
LIMIT $2 OFFSET $3;

-- name: GetDailySalesTrend :many
SELECT 
    DATE(created_at) as sale_date,
    COALESCE(SUM(total_amount), 0)::numeric as total_revenue,
    COALESCE(SUM((SELECT SUM(quantity) FROM order_items WHERE order_items.order_id = orders.id)), 0)::bigint as tickets_sold
FROM orders 
WHERE event_id = ANY($1::uuid[]) AND status = 'SUKSES'
  AND created_at >= NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at)
ORDER BY sale_date ASC;
