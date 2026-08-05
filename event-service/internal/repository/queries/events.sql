-- name: CreateEvent :one
INSERT INTO events (
    organizer_id, venue_id, category_id, title, slug, description,
    banner_url, start_date, end_date, status, is_online, online_url, max_attendees
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetEventByID :one
SELECT * FROM events WHERE id = $1;

-- name: GetEventBySlug :one
SELECT * FROM events WHERE slug = $1;

-- name: ListEvents :many
SELECT * FROM events
WHERE status = 'published'
ORDER BY start_date ASC
LIMIT $1 OFFSET $2;

-- name: ListEventsByOrganizer :many
SELECT * FROM events
WHERE organizer_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListEventsByCategory :many
SELECT * FROM events
WHERE category_id = $1 AND status = 'published'
ORDER BY start_date ASC
LIMIT $2 OFFSET $3;

-- name: UpdateEvent :one
UPDATE events SET
    venue_id = $2, category_id = $3, title = $4, slug = $5,
    description = $6, banner_url = $7, start_date = $8, end_date = $9,
    status = $10, is_online = $11, online_url = $12, max_attendees = $13,
    updated_at = NOW()
WHERE id = $1 AND organizer_id = $14
RETURNING *;

-- name: UpdateEventStatus :one
UPDATE events SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1 AND organizer_id = $2;

-- name: CountPublishedEvents :one
SELECT COUNT(*) FROM events WHERE status = 'published';

-- name: CountEventsByOrganizer :one
SELECT COUNT(*) FROM events WHERE organizer_id = $1;

-- name: SearchEvents :many
SELECT * FROM events
WHERE status = 'published'
  AND (title ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%')
ORDER BY start_date ASC
LIMIT $2 OFFSET $3;

-- name: ListEventIDsByOrganizer :many
SELECT id FROM events WHERE organizer_id = $1;
