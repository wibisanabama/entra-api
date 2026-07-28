package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"entra-api/event-service/internal/repository/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrEventNotFound   = errors.New("event not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrSlugExists      = errors.New("slug already exists")
)

type CreateEventRequest struct {
	VenueID      string `json:"venue_id"`
	CategoryID   string `json:"category_id"`
	Title        string `json:"title" binding:"required,min=3"`
	Description  string `json:"description"`
	BannerURL    string `json:"banner_url"`
	StartDate    string `json:"start_date" binding:"required"`
	EndDate      string `json:"end_date" binding:"required"`
	IsOnline     bool   `json:"is_online"`
	OnlineURL    string `json:"online_url"`
	MaxAttendees int32  `json:"max_attendees"`
}

type UpdateEventRequest struct {
	VenueID      string `json:"venue_id"`
	CategoryID   string `json:"category_id"`
	Title        string `json:"title" binding:"required,min=3"`
	Description  string `json:"description"`
	BannerURL    string `json:"banner_url"`
	StartDate    string `json:"start_date" binding:"required"`
	EndDate      string `json:"end_date" binding:"required"`
	Status       string `json:"status"`
	IsOnline     bool   `json:"is_online"`
	OnlineURL    string `json:"online_url"`
	MaxAttendees int32  `json:"max_attendees"`
}

type EventService struct {
	queries *db.Queries
}

func NewEventService(queries *db.Queries) *EventService {
	return &EventService{queries: queries}
}

func (s *EventService) CreateEvent(ctx context.Context, organizerID string, req CreateEventRequest) (*db.Event, error) {
	orgUUID, err := uuid.Parse(organizerID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date format: %w", err)
	}
	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date format: %w", err)
	}

	slug := generateSlug(req.Title)

	event, err := s.queries.CreateEvent(ctx, db.CreateEventParams{
		OrganizerID:  orgUUID,
		VenueID:      pgUUIDFromString(req.VenueID),
		CategoryID:   pgUUIDFromString(req.CategoryID),
		Title:        req.Title,
		Slug:         slug,
		Description:  pgTextFromString(req.Description),
		BannerUrl:    pgTextFromString(req.BannerURL),
		StartDate:    pgTimestamptzFromTime(startDate),
		EndDate:      pgTimestamptzFromTime(endDate),
		Status:       "draft",
		IsOnline:     req.IsOnline,
		OnlineUrl:    pgTextFromString(req.OnlineURL),
		MaxAttendees: pgInt4FromInt32(req.MaxAttendees),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	return &event, nil
}

func (s *EventService) GetEvent(ctx context.Context, eventID string) (*db.Event, error) {
	id, err := uuid.Parse(eventID)
	if err != nil {
		return nil, ErrEventNotFound
	}

	event, err := s.queries.GetEventByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return &event, nil
}

func (s *EventService) ListEvents(ctx context.Context, page, perPage int) ([]db.Event, int64, error) {
	offset := (page - 1) * perPage

	events, err := s.queries.ListEvents(ctx, int32(perPage), int32(offset))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list events: %w", err)
	}

	total, err := s.queries.CountPublishedEvents(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count events: %w", err)
	}

	return events, total, nil
}

func (s *EventService) UpdateEvent(ctx context.Context, eventID, organizerID string, req UpdateEventRequest) (*db.Event, error) {
	id, err := uuid.Parse(eventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	orgUUID, err := uuid.Parse(organizerID)
	if err != nil {
		return nil, ErrUnauthorized
	}

	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date format: %w", err)
	}
	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date format: %w", err)
	}

	status := req.Status
	if status == "" {
		status = "draft"
	}

	slug := generateSlug(req.Title)

	event, err := s.queries.UpdateEvent(ctx, db.UpdateEventParams{
		ID:           id,
		VenueID:      pgUUIDFromString(req.VenueID),
		CategoryID:   pgUUIDFromString(req.CategoryID),
		Title:        req.Title,
		Slug:         slug,
		Description:  pgTextFromString(req.Description),
		BannerUrl:    pgTextFromString(req.BannerURL),
		StartDate:    pgTimestamptzFromTime(startDate),
		EndDate:      pgTimestamptzFromTime(endDate),
		Status:       status,
		IsOnline:     req.IsOnline,
		OnlineUrl:    pgTextFromString(req.OnlineURL),
		MaxAttendees: pgInt4FromInt32(req.MaxAttendees),
		OrganizerID:  orgUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	return &event, nil
}

func (s *EventService) DeleteEvent(ctx context.Context, eventID, organizerID string) error {
	id, err := uuid.Parse(eventID)
	if err != nil {
		return ErrEventNotFound
	}
	orgUUID, err := uuid.Parse(organizerID)
	if err != nil {
		return ErrUnauthorized
	}

	return s.queries.DeleteEvent(ctx, id, orgUUID)
}

func (s *EventService) SearchEvents(ctx context.Context, query string, page, perPage int) ([]db.Event, error) {
	offset := (page - 1) * perPage
	return s.queries.SearchEvents(ctx, query, int32(perPage), int32(offset))
}

// Helper functions

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Add a short UUID suffix for uniqueness
	shortID := uuid.New().String()[:8]
	return slug + "-" + shortID
}

func pgTextFromString(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgUUIDFromString(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{Valid: false}
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgTimestamptzFromTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func pgInt4FromInt32(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: v, Valid: true}
}
