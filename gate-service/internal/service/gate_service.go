package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"entra-api/gate-service/internal/repository/db"
	"entra-api/shared/kafka"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

func pgUUIDFromUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: u != uuid.Nil}
}

func uuidFromPgUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func (s *GateService) SyncTicket(ctx context.Context, ticketID uuid.UUID, eventID uuid.UUID, ticketCode string, status string) error {
	_, err := s.queries.CreateLocalTicket(ctx, db.CreateLocalTicketParams{
		ID:         pgUUIDFromUUID(ticketID),
		EventID:    pgUUIDFromUUID(eventID),
		TicketCode: ticketCode,
		Status:     status,
	})
	if err != nil {
		// If already exists, update status
		_, _ = s.queries.UpdateLocalTicketStatus(ctx, db.UpdateLocalTicketStatusParams{
			ID:     pgUUIDFromUUID(ticketID),
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
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/internal/tickets/code/%s", ticketServiceURL, ticketCode), nil)
	if reqErr == nil {
		if secret := os.Getenv("INTERNAL_SERVICE_SECRET"); secret != "" {
			req.Header.Set("X-Internal-Secret", secret)
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, httpErr := client.Do(req)
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
					parsedEventID, _ := uuid.Parse(res.Data.EventID)
					_ = s.SyncTicket(ctx, parsedID, parsedEventID, codeToSync, status)
				}
			}
			resp.Body.Close()
		}
	}

	// Local DB check
	var ticket db.LocalTicket
	var err error
	ticket, err = s.queries.GetLocalTicketByCode(ctx, ticketCode)
	if err != nil {
		if parsedUUID, parseErr := uuid.Parse(ticketCode); parseErr == nil {
			ticket, err = s.queries.GetLocalTicketByID(ctx, pgUUIDFromUUID(parsedUUID))
		}
	}

	if err != nil {
		return errors.New("ticket not found")
	}

	// Verify event_id in local mode if eventID is provided
	if eventID != "" && ticket.EventID.Valid {
		if parsedEventUUID, parseErr := uuid.Parse(eventID); parseErr == nil && uuidFromPgUUID(ticket.EventID) != parsedEventUUID {
			return errors.New("ticket belongs to another event")
		}
	}

	if ticket.Status == "CHECKED_IN" || ticket.Status == "USED" {
		return errors.New("ticket already used or invalid")
	}

	updatedTicket, err := s.queries.UpdateLocalTicketStatus(ctx, db.UpdateLocalTicketStatusParams{
		ID:     ticket.ID,
		Status: "CHECKED_IN",
	})
	if err != nil {
		return errors.New("ticket already used or invalid")
	}

	ticketUUID := uuidFromPgUUID(updatedTicket.ID)
	payload := map[string]interface{}{
		"ticket_id":   ticketUUID.String(),
		"ticket_code": updatedTicket.TicketCode,
	}
	payloadBytes, _ := json.Marshal(payload)

	if s.producer != nil {
		_ = s.producer.Publish(ctx, "ticket.scanned", []byte(ticketUUID.String()), payloadBytes)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/internal/events/%s/gate-stats", ticketServiceURL, eventID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gate stats request: %w", err)
	}
	if secret := os.Getenv("INTERNAL_SERVICE_SECRET"); secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
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

