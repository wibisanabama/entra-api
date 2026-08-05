package service

import (
	"context"
	"errors"
	"fmt"

	"entra-api/event-service/internal/repository/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrVenueNotFound = errors.New("venue not found")

type CreateVenueRequest struct {
	Name        string  `json:"name" binding:"required,min=3"`
	Address     string  `json:"address" binding:"required"`
	City        string  `json:"city" binding:"required"`
	Province    string  `json:"province"`
	Country     string  `json:"country"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Capacity    int32   `json:"capacity"`
	Description string  `json:"description"`
}

type UpdateVenueRequest struct {
	Name        string  `json:"name" binding:"required,min=3"`
	Address     string  `json:"address" binding:"required"`
	City        string  `json:"city" binding:"required"`
	Province    string  `json:"province"`
	Country     string  `json:"country"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Capacity    int32   `json:"capacity"`
	Description string  `json:"description"`
}

type VenueService struct {
	queries *db.Queries
}

func NewVenueService(queries *db.Queries) *VenueService {
	return &VenueService{queries: queries}
}

func (s *VenueService) CreateVenue(ctx context.Context, organizerID string, req CreateVenueRequest) (*db.Venue, error) {
	pgOrgUUID := pgUUIDFromString(organizerID)
	if !pgOrgUUID.Valid {
		return nil, ErrUnauthorized
	}

	country := req.Country
	if country == "" {
		country = "Indonesia"
	}

	venue, err := s.queries.CreateVenue(ctx, db.CreateVenueParams{
		OrganizerID: pgOrgUUID,
		Name:        req.Name,
		Address:     req.Address,
		City:        req.City,
		Province:    pgTextFromString(req.Province),
		Country:     country,
		Latitude:    pgNumericFromFloat(req.Latitude),
		Longitude:   pgNumericFromFloat(req.Longitude),
		Capacity:    pgInt4FromInt32(req.Capacity),
		Description: pgTextFromString(req.Description),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create venue: %w", err)
	}

	return &venue, nil
}

func (s *VenueService) GetVenue(ctx context.Context, venueID string) (*db.Venue, error) {
	pgID := pgUUIDFromString(venueID)
	if !pgID.Valid {
		return nil, ErrVenueNotFound
	}

	venue, err := s.queries.GetVenueByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVenueNotFound
		}
		return nil, fmt.Errorf("failed to get venue: %w", err)
	}

	return &venue, nil
}

func (s *VenueService) ListVenues(ctx context.Context, page, perPage int) ([]db.Venue, int64, error) {
	offset := (page - 1) * perPage

	venues, err := s.queries.ListVenues(ctx, db.ListVenuesParams{
		Limit:  int32(perPage),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list venues: %w", err)
	}

	total, err := s.queries.CountVenues(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count venues: %w", err)
	}

	return venues, total, nil
}

func (s *VenueService) UpdateVenue(ctx context.Context, venueID, organizerID string, req UpdateVenueRequest) (*db.Venue, error) {
	pgID := pgUUIDFromString(venueID)
	if !pgID.Valid {
		return nil, ErrVenueNotFound
	}
	pgOrgUUID := pgUUIDFromString(organizerID)
	if !pgOrgUUID.Valid {
		return nil, ErrUnauthorized
	}

	country := req.Country
	if country == "" {
		country = "Indonesia"
	}

	venue, err := s.queries.UpdateVenue(ctx, db.UpdateVenueParams{
		ID:          pgID,
		Name:        req.Name,
		Address:     req.Address,
		City:        req.City,
		Province:    pgTextFromString(req.Province),
		Country:     country,
		Latitude:    pgNumericFromFloat(req.Latitude),
		Longitude:   pgNumericFromFloat(req.Longitude),
		Capacity:    pgInt4FromInt32(req.Capacity),
		Description: pgTextFromString(req.Description),
		OrganizerID: pgOrgUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVenueNotFound
		}
		return nil, fmt.Errorf("failed to update venue: %w", err)
	}

	return &venue, nil
}

func (s *VenueService) DeleteVenue(ctx context.Context, venueID, organizerID string) error {
	pgID := pgUUIDFromString(venueID)
	if !pgID.Valid {
		return ErrVenueNotFound
	}
	pgOrgUUID := pgUUIDFromString(organizerID)
	if !pgOrgUUID.Valid {
		return ErrUnauthorized
	}

	err := s.queries.DeleteVenue(ctx, db.DeleteVenueParams{
		ID:          pgID,
		OrganizerID: pgOrgUUID,
	})
	return err
}

func pgNumericFromFloat(f float64) pgtype.Numeric {
	if f == 0 {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}
