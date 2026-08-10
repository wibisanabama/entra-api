package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"entra-api/ticket-service/internal/repository/db"

	"github.com/google/uuid"
)

type EventResponse struct {
	Data []string `json:"data"`
}

func (s *TicketService) FetchOrganizerEventIDs(organizerID string) ([]string, error) {
	eventServiceURL := os.Getenv("EVENT_SERVICE_URL")
	if eventServiceURL == "" {
		eventServiceURL = "http://localhost:8082" // default local development URL
	}

	url := fmt.Sprintf("%s/api/v1/internal/organizer/%s/events", eventServiceURL, organizerID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch events: status %d", resp.StatusCode)
	}

	var res EventResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (s *TicketService) GetDashboardStats(ctx context.Context, organizerID string) (*db.GetOrganizerStatsRow, error) {
	eventIDsStr, err := s.FetchOrganizerEventIDs(organizerID)
	if err != nil {
		return nil, err
	}

	if len(eventIDsStr) == 0 {
		return &db.GetOrganizerStatsRow{}, nil
	}

	var eventIDs []uuid.UUID
	for _, idStr := range eventIDsStr {
		uid, err := uuid.Parse(idStr)
		if err == nil {
			eventIDs = append(eventIDs, uid)
		}
	}

	stats, err := s.queries.GetOrganizerStats(ctx, eventIDs)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

func (s *TicketService) ListOrganizerOrders(ctx context.Context, organizerID string, page, perPage int) ([]db.Order, error) {
	eventIDsStr, err := s.FetchOrganizerEventIDs(organizerID)
	if err != nil {
		return nil, err
	}

	if len(eventIDsStr) == 0 {
		return []db.Order{}, nil
	}

	var eventIDs []uuid.UUID
	for _, idStr := range eventIDsStr {
		uid, err := uuid.Parse(idStr)
		if err == nil {
			eventIDs = append(eventIDs, uid)
		}
	}

	offset := (page - 1) * perPage
	return s.queries.ListOrdersByEvents(ctx, db.ListOrdersByEventsParams{
		Column1: eventIDs,
		Limit:   int32(perPage),
		Offset:  int32(offset),
	})
}

func (s *TicketService) GetSalesTrend(ctx context.Context, organizerID string) ([]db.GetDailySalesTrendRow, error) {
	eventIDsStr, err := s.FetchOrganizerEventIDs(organizerID)
	if err != nil {
		return nil, err
	}

	if len(eventIDsStr) == 0 {
		return []db.GetDailySalesTrendRow{}, nil
	}

	var eventIDs []uuid.UUID
	for _, idStr := range eventIDsStr {
		id, err := uuid.Parse(idStr)
		if err == nil {
			eventIDs = append(eventIDs, id)
		}
	}

	return s.queries.GetDailySalesTrend(ctx, eventIDs)
}

func (s *TicketService) GetOrganizerOrder(ctx context.Context, orderID string, organizerID string) (*db.Order, []db.OrderItem, []db.Ticket, error) {
	parsedOrderID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid order id: %w", err)
	}

	order, err := s.queries.GetOrder(ctx, parsedOrderID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get order: %w", err)
	}

	eventIDsStr, err := s.FetchOrganizerEventIDs(organizerID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch organizer events: %w", err)
	}

	isOwner := false
	for _, idStr := range eventIDsStr {
		if order.EventID.String() == idStr {
			isOwner = true
			break
		}
	}

	if !isOwner {
		return nil, nil, nil, fmt.Errorf("unauthorized to access this order")
	}

	items, err := s.queries.ListOrderItems(ctx, parsedOrderID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list order items: %w", err)
	}

	tickets, err := s.queries.ListTicketsByOrder(ctx, parsedOrderID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return &order, items, tickets, nil
}

func (s *TicketService) GetEventAttendees(ctx context.Context, eventID string, organizerID string) ([]db.Ticket, error) {
	parsedEventID, err := uuid.Parse(eventID)
	if err != nil {
		return nil, fmt.Errorf("invalid event id: %w", err)
	}

	eventIDsStr, err := s.FetchOrganizerEventIDs(organizerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch organizer events: %w", err)
	}

	isOwner := false
	for _, idStr := range eventIDsStr {
		if eventID == idStr {
			isOwner = true
			break
		}
	}

	if !isOwner {
		return nil, fmt.Errorf("unauthorized to access this event")
	}

	tickets, err := s.queries.ListTicketsByEvent(ctx, parsedEventID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return tickets, nil
}
