-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token, user_agent, ip_address, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens WHERE token = $1 AND expires_at > NOW();

-- name: DeleteRefreshToken :exec
DELETE FROM refresh_tokens WHERE token = $1;

-- name: DeleteRefreshTokensByUserID :exec
DELETE FROM refresh_tokens WHERE user_id = $1;

-- name: DeleteExpiredTokens :exec
DELETE FROM refresh_tokens WHERE expires_at <= NOW();
