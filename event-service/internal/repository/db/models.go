package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Category struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Slug      string      `json:"slug"`
	Icon      pgtype.Text `json:"icon"`
	CreatedAt time.Time   `json:"created_at"`
}

type Venue struct {
	ID          uuid.UUID      `json:"id"`
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
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Event struct {
	ID           uuid.UUID   `json:"id"`
	OrganizerID  uuid.UUID   `json:"organizer_id"`
	VenueID      pgtype.UUID `json:"venue_id"`
	CategoryID   pgtype.UUID `json:"category_id"`
	Title        string      `json:"title"`
	Slug         string      `json:"slug"`
	Description  pgtype.Text `json:"description"`
	BannerUrl    pgtype.Text `json:"banner_url"`
	StartDate    time.Time   `json:"start_date"`
	EndDate      time.Time   `json:"end_date"`
	Status       string      `json:"status"`
	IsOnline     bool        `json:"is_online"`
	OnlineUrl    pgtype.Text `json:"online_url"`
	MaxAttendees pgtype.Int4 `json:"max_attendees"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type TicketType struct {
	ID          uuid.UUID      `json:"id"`
	EventID     uuid.UUID      `json:"event_id"`
	Name        string         `json:"name"`
	Description pgtype.Text    `json:"description"`
	Price       pgtype.Numeric `json:"price"`
	Quantity    int32          `json:"quantity"`
	Sold        int32          `json:"sold"`
	MaxPerOrder pgtype.Int4    `json:"max_per_order"`
	SaleStart   pgtype.Timestamptz `json:"sale_start"`
	SaleEnd     pgtype.Timestamptz `json:"sale_end"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
