-- name: CreateWallet :one
INSERT INTO wallets (user_id) VALUES ($1) RETURNING *;

-- name: GetWalletByUserID :one
SELECT * FROM wallets WHERE user_id = $1;

-- name: UpdateWalletBalance :one
UPDATE wallets SET balance = balance + $2, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: DeductWalletBalance :one
UPDATE wallets SET balance = balance - $2, updated_at = NOW() WHERE id = $1 AND balance >= $2 RETURNING *;

-- name: CreateTopup :one
INSERT INTO topups (wallet_id, amount, status) VALUES ($1, $2, $3) RETURNING *;

-- name: GetTopup :one
SELECT * FROM topups WHERE id = $1;

-- name: UpdateTopupStatus :one
UPDATE topups SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: CreateTransaction :one
INSERT INTO transactions (wallet_id, type, amount, merchant_id, description) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListTransactions :many
SELECT * FROM transactions WHERE wallet_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;
