package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"entra-api/ticket-service/internal/service"

	"github.com/IBM/sarama"
)

type PaymentConsumer struct {
	ticketService *service.TicketService
}

func NewPaymentConsumer(ticketService *service.TicketService) *PaymentConsumer {
	return &PaymentConsumer{ticketService: ticketService}
}

func (c *PaymentConsumer) HandleMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	switch message.Topic {
	case "payment.success":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			refType, _ := payload["reference_type"].(string)
			if refType == "TICKET" {
				if orderID, ok := payload["reference_id"].(string); ok {
					err := c.ticketService.HandlePaymentSuccess(ctx, orderID)
					if err != nil {
						return err
					}
					slog.Info("processed payment success, tickets generated", "order_id", orderID)
				}
			}
		}
	case "payment.failed":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			refType, _ := payload["reference_type"].(string)
			if refType == "TICKET" {
				if orderID, ok := payload["reference_id"].(string); ok {
					err := c.ticketService.HandlePaymentFailed(ctx, orderID)
					if err != nil {
						return err
					}
					slog.Info("processed payment failed, order cancelled", "order_id", orderID)
				}
			}
		}
	}
	return nil
}
