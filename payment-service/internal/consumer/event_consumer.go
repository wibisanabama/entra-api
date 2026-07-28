package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"entra-api/payment-service/internal/service"

	"github.com/IBM/sarama"
)

type EventConsumer struct {
	paymentService *service.PaymentService
}

func NewEventConsumer(paymentService *service.PaymentService) *EventConsumer {
	return &EventConsumer{paymentService: paymentService}
}

func (c *EventConsumer) HandleMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	switch message.Topic {
	case "order.created":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			orderID, _ := payload["order_id"].(string)
			userID, _ := payload["user_id"].(string)
			amount, _ := payload["amount"].(float64)
			_ = c.paymentService.CreatePaymentIntent(ctx, orderID, "TICKET", userID, amount)
			slog.Info("created payment intent for ticket order", "order_id", orderID)
		}
	case "order.cancelled":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			orderID, _ := payload["order_id"].(string)
			_ = c.paymentService.HandleReferenceCancelled(ctx, orderID, "TICKET")
			slog.Info("marked payment intent as expired for cancelled order", "order_id", orderID)
		}
	case "topup.created":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			refID, _ := payload["reference_id"].(string)
			userID, _ := payload["user_id"].(string)
			amount, _ := payload["amount"].(float64)
			_ = c.paymentService.CreatePaymentIntent(ctx, refID, "TOPUP", userID, amount)
			slog.Info("created payment intent for wallet topup", "reference_id", refID)
		}
	}
	return nil
}
