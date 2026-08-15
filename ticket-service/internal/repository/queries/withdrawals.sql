-- name: CreateWithdrawal :one
INSERT INTO withdrawals (
    organizer_id,
    amount,
    bank_name,
    account_number,
    account_name,
    notes,
    status
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetWithdrawal :one
SELECT * FROM withdrawals
WHERE id = $1;

-- name: ListWithdrawalsByOrganizer :many
SELECT * FROM withdrawals
WHERE organizer_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetTotalWithdrawnByOrganizer :one
SELECT 
    COALESCE(SUM(amount), 0)::numeric as total_withdrawn
FROM withdrawals
WHERE organizer_id = $1
  AND status IN ('PENDING', 'APPROVED', 'PAID');

-- name: GetWithdrawnSummaryByOrganizer :one
SELECT 
    COALESCE(SUM(CASE WHEN status IN ('PENDING', 'APPROVED', 'PAID') THEN amount ELSE 0 END), 0)::numeric as total_deducted,
    COALESCE(SUM(CASE WHEN status = 'PENDING' THEN amount ELSE 0 END), 0)::numeric as pending_amount,
    COALESCE(SUM(CASE WHEN status = 'PAID' THEN amount ELSE 0 END), 0)::numeric as paid_amount,
    COALESCE(COUNT(*), 0)::bigint as total_requests
FROM withdrawals
WHERE organizer_id = $1;

-- name: UpdateWithdrawalStatus :one
UPDATE withdrawals
SET 
    status = $2,
    rejection_reason = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListAllWithdrawals :many
SELECT * FROM withdrawals
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListWithdrawalsByStatus :many
SELECT * FROM withdrawals
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
