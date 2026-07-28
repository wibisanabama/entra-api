package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"entra-api/ticket-service/internal/repository/db"

	"github.com/IBM/sarama"
)

type EventConsumer struct {
	queries *db.Queries
}

func NewEventConsumer(queries *db.Queries) *EventConsumer {
	return &EventConsumer{queries: queries}
}

func (c *EventConsumer) HandleMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	switch message.Topic {
	case "ticket.scanned":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			if ticketCode, ok := payload["ticket_code"].(string); ok {
				// Mark ticket as USED
				ticket, err := c.queries.GetTicketByCode(ctx, ticketCode)
				if err == nil {
					_, _ = c.queries.UpdateTicketStatus(ctx, ticket.ID, "USED")
					slog.Info("ticket marked as USED centrally", "ticket_code", ticketCode)
				} else {
					slog.Error("failed to find ticket by code", "ticket_code", ticketCode, "error", err)
				}
			}
		}
	}
	return nil
}
