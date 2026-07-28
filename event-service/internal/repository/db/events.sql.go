package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateEventParams struct {
	OrganizerID  uuid.UUID   `json:"organizer_id"`
	VenueID      pgtype.UUID `json:"venue_id"`
	CategoryID   pgtype.UUID `json:"category_id"`
	Title        string      `json:"title"`
	Slug         string      `json:"slug"`
	Description  pgtype.Text `json:"description"`
	BannerUrl    pgtype.Text `json:"banner_url"`
	StartDate    pgtype.Timestamptz `json:"start_date"`
	EndDate      pgtype.Timestamptz `json:"end_date"`
	Status       string      `json:"status"`
	IsOnline     bool        `json:"is_online"`
	OnlineUrl    pgtype.Text `json:"online_url"`
	MaxAttendees pgtype.Int4 `json:"max_attendees"`
}

func (q *Queries) CreateEvent(ctx context.Context, arg CreateEventParams) (Event, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO events (organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at`,
		arg.OrganizerID, arg.VenueID, arg.CategoryID, arg.Title, arg.Slug, arg.Description, arg.BannerUrl, arg.StartDate, arg.EndDate, arg.Status, arg.IsOnline, arg.OnlineUrl, arg.MaxAttendees,
	)
	var i Event
	err := row.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetEventByID(ctx context.Context, id uuid.UUID) (Event, error) {
	row := q.db.QueryRow(ctx, `SELECT id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at FROM events WHERE id = $1`, id)
	var i Event
	err := row.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetEventBySlug(ctx context.Context, slug string) (Event, error) {
	row := q.db.QueryRow(ctx, `SELECT id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at FROM events WHERE slug = $1`, slug)
	var i Event
	err := row.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) ListEvents(ctx context.Context, limit int32, offset int32) ([]Event, error) {
	rows, err := q.db.Query(ctx, `SELECT id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at FROM events WHERE status = 'published' ORDER BY start_date ASC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) ListEventsByOrganizer(ctx context.Context, organizerID uuid.UUID, limit int32, offset int32) ([]Event, error) {
	rows, err := q.db.Query(ctx, `SELECT id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at FROM events WHERE organizer_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, organizerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) ListEventsByCategory(ctx context.Context, categoryID uuid.UUID, limit int32, offset int32) ([]Event, error) {
	rows, err := q.db.Query(ctx, `SELECT id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at FROM events WHERE category_id = $1 AND status = 'published' ORDER BY start_date ASC LIMIT $2 OFFSET $3`, categoryID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type UpdateEventParams struct {
	ID           uuid.UUID   `json:"id"`
	VenueID      pgtype.UUID `json:"venue_id"`
	CategoryID   pgtype.UUID `json:"category_id"`
	Title        string      `json:"title"`
	Slug         string      `json:"slug"`
	Description  pgtype.Text `json:"description"`
	BannerUrl    pgtype.Text `json:"banner_url"`
	StartDate    pgtype.Timestamptz `json:"start_date"`
	EndDate      pgtype.Timestamptz `json:"end_date"`
	Status       string      `json:"status"`
	IsOnline     bool        `json:"is_online"`
	OnlineUrl    pgtype.Text `json:"online_url"`
	MaxAttendees pgtype.Int4 `json:"max_attendees"`
	OrganizerID  uuid.UUID   `json:"organizer_id"`
}

func (q *Queries) UpdateEvent(ctx context.Context, arg UpdateEventParams) (Event, error) {
	row := q.db.QueryRow(ctx,
		`UPDATE events SET venue_id = $2, category_id = $3, title = $4, slug = $5, description = $6, banner_url = $7, start_date = $8, end_date = $9, status = $10, is_online = $11, online_url = $12, max_attendees = $13, updated_at = NOW() WHERE id = $1 AND organizer_id = $14 RETURNING id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at`,
		arg.ID, arg.VenueID, arg.CategoryID, arg.Title, arg.Slug, arg.Description, arg.BannerUrl, arg.StartDate, arg.EndDate, arg.Status, arg.IsOnline, arg.OnlineUrl, arg.MaxAttendees, arg.OrganizerID,
	)
	var i Event
	err := row.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) UpdateEventStatus(ctx context.Context, id uuid.UUID, status string) (Event, error) {
	row := q.db.QueryRow(ctx, `UPDATE events SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at`, id, status)
	var i Event
	err := row.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) DeleteEvent(ctx context.Context, id uuid.UUID, organizerID uuid.UUID) error {
	_, err := q.db.Exec(ctx, `DELETE FROM events WHERE id = $1 AND organizer_id = $2`, id, organizerID)
	return err
}

func (q *Queries) CountPublishedEvents(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE status = 'published'`)
	var count int64
	err := row.Scan(&count)
	return count, err
}

func (q *Queries) CountEventsByOrganizer(ctx context.Context, organizerID uuid.UUID) (int64, error) {
	row := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM events WHERE organizer_id = $1`, organizerID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

func (q *Queries) SearchEvents(ctx context.Context, query string, limit int32, offset int32) ([]Event, error) {
	rows, err := q.db.Query(ctx, `SELECT id, organizer_id, venue_id, category_id, title, slug, description, banner_url, start_date, end_date, status, is_online, online_url, max_attendees, created_at, updated_at FROM events WHERE status = 'published' AND (title ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%') ORDER BY start_date ASC LIMIT $2 OFFSET $3`, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var i Event
		if err := rows.Scan(&i.ID, &i.OrganizerID, &i.VenueID, &i.CategoryID, &i.Title, &i.Slug, &i.Description, &i.BannerUrl, &i.StartDate, &i.EndDate, &i.Status, &i.IsOnline, &i.OnlineUrl, &i.MaxAttendees, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}
