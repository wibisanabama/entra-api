package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateVenueParams struct {
	OrganizerID uuid.UUID      `json:"organizer_id"`
	Name        string         `json:"name"`
	Address     string         `json:"address"`
	City        string         `json:"city"`
	Province    pgtype.Text    `json:"province"`
	Country     string         `json:"country"`
	Latitude    pgtype.Numeric `json:"latitude"`
	Longitude   pgtype.Numeric `json:"longitude"`
	Capacity    pgtype.Int4    `json:"capacity"`
	Description pgtype.Text    `json:"description"`
}

func (q *Queries) CreateVenue(ctx context.Context, arg CreateVenueParams) (Venue, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO venues (organizer_id, name, address, city, province, country, latitude, longitude, capacity, description) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, organizer_id, name, address, city, province, country, latitude, longitude, capacity, description, created_at, updated_at`,
		arg.OrganizerID, arg.Name, arg.Address, arg.City, arg.Province, arg.Country, arg.Latitude, arg.Longitude, arg.Capacity, arg.Description,
	)
	var i Venue
	err := row.Scan(&i.ID, &i.OrganizerID, &i.Name, &i.Address, &i.City, &i.Province, &i.Country, &i.Latitude, &i.Longitude, &i.Capacity, &i.Description, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetVenueByID(ctx context.Context, id uuid.UUID) (Venue, error) {
	row := q.db.QueryRow(ctx, `SELECT id, organizer_id, name, address, city, province, country, latitude, longitude, capacity, description, created_at, updated_at FROM venues WHERE id = $1`, id)
	var i Venue
	err := row.Scan(&i.ID, &i.OrganizerID, &i.Name, &i.Address, &i.City, &i.Province, &i.Country, &i.Latitude, &i.Longitude, &i.Capacity, &i.Description, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) ListVenues(ctx context.Context, limit int32, offset int32) ([]Venue, error) {
	rows, err := q.db.Query(ctx, `SELECT id, organizer_id, name, address, city, province, country, latitude, longitude, capacity, description, created_at, updated_at FROM venues ORDER BY name ASC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Venue{}
	for rows.Next() {
		var i Venue
		if err := rows.Scan(&i.ID, &i.OrganizerID, &i.Name, &i.Address, &i.City, &i.Province, &i.Country, &i.Latitude, &i.Longitude, &i.Capacity, &i.Description, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) ListVenuesByOrganizer(ctx context.Context, organizerID uuid.UUID, limit int32, offset int32) ([]Venue, error) {
	rows, err := q.db.Query(ctx, `SELECT id, organizer_id, name, address, city, province, country, latitude, longitude, capacity, description, created_at, updated_at FROM venues WHERE organizer_id = $1 ORDER BY name ASC LIMIT $2 OFFSET $3`, organizerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Venue{}
	for rows.Next() {
		var i Venue
		if err := rows.Scan(&i.ID, &i.OrganizerID, &i.Name, &i.Address, &i.City, &i.Province, &i.Country, &i.Latitude, &i.Longitude, &i.Capacity, &i.Description, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type UpdateVenueParams struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Address     string         `json:"address"`
	City        string         `json:"city"`
	Province    pgtype.Text    `json:"province"`
	Country     string         `json:"country"`
	Latitude    pgtype.Numeric `json:"latitude"`
	Longitude   pgtype.Numeric `json:"longitude"`
	Capacity    pgtype.Int4    `json:"capacity"`
	Description pgtype.Text    `json:"description"`
	OrganizerID uuid.UUID      `json:"organizer_id"`
}

func (q *Queries) UpdateVenue(ctx context.Context, arg UpdateVenueParams) (Venue, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE venues SET name = $2, address = $3, city = $4, province = $5, country = $6, latitude = $7, longitude = $8, capacity = $9, description = $10, updated_at = NOW() WHERE id = $1 AND organizer_id = $11 RETURNING id, organizer_id, name, address, city, province, country, latitude, longitude, capacity, description, created_at, updated_at`,
		arg.ID, arg.Name, arg.Address, arg.City, arg.Province, arg.Country, arg.Latitude, arg.Longitude, arg.Capacity, arg.Description, arg.OrganizerID,
	)
	var i Venue
	err := row.Scan(&i.ID, &i.OrganizerID, &i.Name, &i.Address, &i.City, &i.Province, &i.Country, &i.Latitude, &i.Longitude, &i.Capacity, &i.Description, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) DeleteVenue(ctx context.Context, id uuid.UUID, organizerID uuid.UUID) error {
	_, err := q.db.Exec(ctx, `DELETE FROM venues WHERE id = $1 AND organizer_id = $2`, id, organizerID)
	return err
}

func (q *Queries) CountVenues(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM venues`)
	var count int64
	err := row.Scan(&count)
	return count, err
}
