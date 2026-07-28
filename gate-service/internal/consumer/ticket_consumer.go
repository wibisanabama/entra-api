package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"entra-api/gate-service/internal/service"
	"github.com/google/uuid"
	"github.com/IBM/sarama"
)

type TicketPayload struct {
	TicketID   uuid.UUID `json:"ticket_id"`
	TicketCode string    `json:"ticket_code"`
	Status     string    `json:"status"`
}

type TicketConsumer struct {
	gateService *service.GateService
}

func NewTicketConsumer(gateService *service.GateService) *TicketConsumer {
	return &TicketConsumer{
		gateService: gateService,
	}
}

func (c *TicketConsumer) HandleMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	var payload TicketPayload
	if err := json.Unmarshal(message.Value, &payload); err != nil {
		slog.Error("Failed to unmarshal ticket message", "error", err)
		return nil // Return nil to avoid blocking on bad messages
	}

	err := c.gateService.SyncTicket(ctx, payload.TicketID, payload.TicketCode, payload.Status)
	if err != nil {
		slog.Error("Failed to sync ticket", "error", err)
		return err // We might want to retry
	}

	slog.Info("Successfully synced ticket", "ticket_code", payload.TicketCode)
	return nil
}
