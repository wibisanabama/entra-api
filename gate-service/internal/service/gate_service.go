package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"entra-api/gate-service/internal/repository/db"
	"entra-api/shared/kafka"
	"github.com/google/uuid"
)

type GateService struct {
	queries  *db.Queries
	producer *kafka.Producer
}

func NewGateService(queries *db.Queries, producer *kafka.Producer) *GateService {
	return &GateService{
		queries:  queries,
		producer: producer,
	}
}

func (s *GateService) SyncTicket(ctx context.Context, ticketID uuid.UUID, ticketCode string, status string) error {
	_, err := s.queries.CreateLocalTicket(ctx, db.CreateLocalTicketParams{
		ID:         ticketID,
		TicketCode: ticketCode,
		Status:     status,
	})
	if err != nil {
		// If already exists, update status
		_, _ = s.queries.UpdateLocalTicketStatus(ctx, db.UpdateLocalTicketStatusParams{
			ID:     ticketID,
			Status: status,
		})
	}
	return nil
}

func (s *GateService) ScanTicket(ctx context.Context, ticketCode string, eventID string) error {
	ticketServiceURL := os.Getenv("TICKET_SERVICE_URL")
	if ticketServiceURL == "" {
		ticketServiceURL = "http://localhost:8083"
	}

	// Query ticket-service to get latest ticket details and verify event_id ownership
	resp, httpErr := http.Get(fmt.Sprintf("%s/api/v1/internal/tickets/code/%s", ticketServiceURL, ticketCode))
	if httpErr == nil && resp.StatusCode == http.StatusOK {
		var res struct {
			Data struct {
				ID         string `json:"id"`
				EventID    string `json:"event_id"`
				TicketCode string `json:"ticket_code"`
				Status     string `json:"status"`
			} `json:"data"`
		}
		if errDecode := json.NewDecoder(resp.Body).Decode(&res); errDecode == nil && res.Data.ID != "" {
			// Strict Event ID Verification! Rejects tickets belonging to other events.
			if eventID != "" && res.Data.EventID != "" && res.Data.EventID != eventID {
				resp.Body.Close()
				return errors.New("ticket belongs to another event")
			}

			parsedID, parseErr := uuid.Parse(res.Data.ID)
			if parseErr == nil {
				status := res.Data.Status
				if status == "" {
					status = "ACTIVE"
				}
				codeToSync := res.Data.TicketCode
				if codeToSync == "" {
					codeToSync = ticketCode
				}
				_ = s.SyncTicket(ctx, parsedID, codeToSync, status)
			}
		}
		resp.Body.Close()
	}

	// Local DB check
	var ticket db.LocalTicket
	var err error
	ticket, err = s.queries.GetLocalTicketByCode(ctx, ticketCode)
	if err != nil {
		if parsedUUID, parseErr := uuid.Parse(ticketCode); parseErr == nil {
			ticket, err = s.queries.GetLocalTicketByID(ctx, parsedUUID)
		}
	}

	if err != nil {
		return errors.New("ticket not found")
	}

	if ticket.Status == "CHECKED_IN" || ticket.Status == "USED" {
		return errors.New("ticket already used or invalid")
	}

	updatedTicket, err := s.queries.UpdateLocalTicketStatus(ctx, db.UpdateLocalTicketStatusParams{
		ID:     ticket.ID,
		Status: "CHECKED_IN",
	})
	if err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}

	payload := map[string]interface{}{
		"ticket_id":   updatedTicket.ID,
		"ticket_code": updatedTicket.TicketCode,
	}
	payloadBytes, _ := json.Marshal(payload)

	if s.producer != nil {
		_ = s.producer.Publish(ctx, "ticket.scanned", []byte(updatedTicket.ID.String()), payloadBytes)
	}

	return nil
}

type GateStatsResponse struct {
	EventID      string  `json:"event_id"`
	TotalTickets int     `json:"total_tickets"`
	CheckedIn    int     `json:"checked_in"`
	Remaining    int     `json:"remaining"`
	CheckInRate  float64 `json:"checkin_rate"`
	Status       string  `json:"status"`
}

func (s *GateService) GetGateStats(ctx context.Context, eventID string) (*GateStatsResponse, error) {
	ticketServiceURL := os.Getenv("TICKET_SERVICE_URL")
	if ticketServiceURL == "" {
		ticketServiceURL = "http://localhost:8083"
	}

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/internal/events/%s/gate-stats", ticketServiceURL, eventID))
	if err != nil {
		return nil, fmt.Errorf("failed to reach ticket service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticket service returned status: %d", resp.StatusCode)
	}

	var res struct {
		Data GateStatsResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode gate stats response: %w", err)
	}

	return &res.Data, nil
}

