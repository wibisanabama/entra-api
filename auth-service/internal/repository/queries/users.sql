-- name: CreateUser :one
INSERT INTO users (email, password_hash, full_name, phone, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND is_active = TRUE;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND is_active = TRUE;

-- name: UpdateUserProfile :one
UPDATE users
SET full_name = $2, phone = $3, avatar_url = $4, updated_at = NOW()
WHERE id = $1 AND is_active = TRUE
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeactivateUser :exec
UPDATE users SET is_active = FALSE, updated_at = NOW() WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users
WHERE is_active = TRUE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users WHERE is_active = TRUE;
