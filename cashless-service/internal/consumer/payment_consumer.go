package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"entra-api/cashless-service/internal/service"

	"github.com/IBM/sarama"
)

type PaymentConsumer struct {
	walletService *service.WalletService
}

func NewPaymentConsumer(walletService *service.WalletService) *PaymentConsumer {
	return &PaymentConsumer{walletService: walletService}
}

func (c *PaymentConsumer) HandleMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	switch message.Topic {
	case "payment.success":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			refType, _ := payload["reference_type"].(string)
			if refType == "TOPUP" {
				refID, _ := payload["reference_id"].(string)
				_ = c.walletService.ProcessTopUpSuccess(ctx, refID)
				slog.Info("processed topup success", "reference_id", refID)
			}
		}
	case "payment.failed":
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Value, &payload); err == nil {
			refType, _ := payload["reference_type"].(string)
			if refType == "TOPUP" {
				refID, _ := payload["reference_id"].(string)
				_ = c.walletService.ProcessTopUpFailed(ctx, refID)
				slog.Info("processed topup failure", "reference_id", refID)
			}
		}
	}
	return nil
}
