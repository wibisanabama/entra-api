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
		return fmt.Errorf("failed to create local ticket: %w", err)
	}
	return nil
}

func (s *GateService) ScanTicket(ctx context.Context, ticketCode string) error {
	ticket, err := s.queries.GetLocalTicketByCode(ctx, ticketCode)
	if err != nil {
		// Fallback: Query ticket-service directly if ticket is not yet synced in local gate DB
		ticketServiceURL := os.Getenv("TICKET_SERVICE_URL")
		if ticketServiceURL == "" {
			ticketServiceURL = "http://localhost:8083"
		}

		resp, httpErr := http.Get(fmt.Sprintf("%s/api/v1/internal/tickets/code/%s", ticketServiceURL, ticketCode))
		if httpErr == nil && resp.StatusCode == http.StatusOK {
			var res struct {
				Data struct {
					ID         string `json:"id"`
					TicketCode string `json:"ticket_code"`
					Status     string `json:"status"`
				} `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.Data.TicketCode != "" {
				parsedID, parseErr := uuid.Parse(res.Data.ID)
				if parseErr == nil {
					status := res.Data.Status
					if status == "" {
						status = "ACTIVE"
					}
					// Sync ticket to local gate DB
					_ = s.SyncTicket(ctx, parsedID, res.Data.TicketCode, status)
					// Retry local query
					ticket, err = s.queries.GetLocalTicketByCode(ctx, ticketCode)
				}
			}
			resp.Body.Close()
		}

		if err != nil {
			return fmt.Errorf("ticket not found: %w", err)
		}
	}

	if ticket.Status != "ACTIVE" {
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

	_ = s.producer.Publish(ctx, "ticket.scanned", []byte(updatedTicket.ID.String()), payloadBytes)

	return nil
}
