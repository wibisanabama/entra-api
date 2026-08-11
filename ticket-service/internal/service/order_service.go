package service

import (
	"bytes"
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

type AttendeeResponse struct {
	ID           uuid.UUID              `json:"id"`
	OrderID      uuid.UUID              `json:"order_id"`
	UserID       uuid.UUID              `json:"user_id"`
	EventID      uuid.UUID              `json:"event_id"`
	TicketTypeID uuid.UUID              `json:"ticket_type_id"`
	TicketCode   string                 `json:"ticket_code"`
	Status       string                 `json:"status"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	UserName     string                 `json:"user_name"`
	UserEmail    string                 `json:"user_email"`
	User         map[string]interface{} `json:"user"`
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

func (s *TicketService) GetEventAttendees(ctx context.Context, eventID string, organizerID string) ([]AttendeeResponse, error) {
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

	// Fetch buyer user details from auth-service
	userIDsMap := make(map[string]bool)
	var userIDs []string
	for _, t := range tickets {
		uidStr := t.UserID.String()
		if !userIDsMap[uidStr] {
			userIDsMap[uidStr] = true
			userIDs = append(userIDs, uidStr)
		}
	}

	userProfiles := make(map[string]struct {
		FullName string
		Email    string
	})

	if len(userIDs) > 0 {
		authServiceURL := os.Getenv("AUTH_SERVICE_URL")
		if authServiceURL == "" {
			authServiceURL = "http://localhost:8081"
		}

		payload, _ := json.Marshal(map[string]interface{}{"ids": userIDs})
		req, err := http.NewRequest(http.MethodPost, authServiceURL+"/api/v1/auth/users/batch", bytes.NewBuffer(payload))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				var res struct {
					Data []struct {
						ID       string `json:"id"`
						Email    string `json:"email"`
						FullName string `json:"full_name"`
					} `json:"data"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
					for _, u := range res.Data {
						userProfiles[u.ID] = struct {
							FullName string
							Email    string
						}{FullName: u.FullName, Email: u.Email}
					}
				}
				resp.Body.Close()
			}
		}
	}

	var result []AttendeeResponse
	for _, t := range tickets {
		prof := userProfiles[t.UserID.String()]
		name := prof.FullName
		if name == "" {
			name = "Pengunjung"
		}
		result = append(result, AttendeeResponse{
			ID:           t.ID,
			OrderID:      t.OrderID,
			UserID:       t.UserID,
			EventID:      t.EventID,
			TicketTypeID: t.TicketTypeID,
			TicketCode:   t.TicketCode,
			Status:       t.Status,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
			UserName:     name,
			UserEmail:    prof.Email,
			User: map[string]interface{}{
				"name":  name,
				"email": prof.Email,
			},
		})
	}

	return result, nil
}

