package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
		return fmt.Errorf("ticket not found: %w", err)
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

	err = s.producer.Publish(ctx, "ticket.scanned", []byte(updatedTicket.ID.String()), payloadBytes)
	if err != nil {
		return fmt.Errorf("failed to publish ticket.scanned event: %w", err)
	}

	return nil
}
