-- name: CreateVenue :one
INSERT INTO venues (
    organizer_id, name, address, city, province, country,
    latitude, longitude, capacity, description
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetVenueByID :one
SELECT * FROM venues WHERE id = $1;

-- name: ListVenues :many
SELECT * FROM venues
ORDER BY name ASC
LIMIT $1 OFFSET $2;

-- name: ListVenuesByOrganizer :many
SELECT * FROM venues
WHERE organizer_id = $1
ORDER BY name ASC
LIMIT $2 OFFSET $3;

-- name: UpdateVenue :one
UPDATE venues SET
    name = $2, address = $3, city = $4, province = $5, country = $6,
    latitude = $7, longitude = $8, capacity = $9, description = $10,
    updated_at = NOW()
WHERE id = $1 AND organizer_id = $11
RETURNING *;

-- name: DeleteVenue :exec
DELETE FROM venues WHERE id = $1 AND organizer_id = $2;

-- name: CountVenues :one
SELECT COUNT(*) FROM venues;
