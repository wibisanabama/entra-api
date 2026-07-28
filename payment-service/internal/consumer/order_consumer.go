package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"entra-api/payment-service/internal/service"

	"github.com/IBM/sarama"
)

type OrderConsumer struct {
	paymentService *service.PaymentService
}

func NewOrderConsumer(paymentService *service.PaymentService) *OrderConsumer {
	return &OrderConsumer{paymentService: paymentService}
}

func (c *OrderConsumer) HandleMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	switch message.Topic {
	case "order.created":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			orderID, _ := payload["order_id"].(string)
			userID, _ := payload["user_id"].(string)
			amount, _ := payload["amount"].(float64)
			_ = c.paymentService.CreatePaymentIntent(ctx, orderID, userID, amount)
			slog.Info("created payment intent for order", "order_id", orderID)
		}
	case "order.cancelled":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			orderID, _ := payload["order_id"].(string)
			_ = c.paymentService.HandleOrderCancelled(ctx, orderID)
			slog.Info("marked payment intent as expired for cancelled order", "order_id", orderID)
		}
	}
	return nil
}
