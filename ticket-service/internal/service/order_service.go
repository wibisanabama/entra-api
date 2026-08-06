package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"entra-api/ticket-service/internal/repository/db"

	"github.com/google/uuid"
)

type EventResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (s *TicketService) FetchOrganizerEventIDs(organizerID string) ([]string, error) {
	url := fmt.Sprintf("http://event-service:8080/api/v1/internal/organizer/%s/events", organizerID)
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

	var ids []string
	for _, event := range res.Data {
		ids = append(ids, event.ID)
	}
	return ids, nil
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
