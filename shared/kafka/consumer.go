package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/IBM/sarama"
)

// MessageHandler is a function that processes a consumed Kafka message.
type MessageHandler func(ctx context.Context, msg *sarama.ConsumerMessage) error

// ConsumerGroup wraps a Sarama ConsumerGroup for consuming messages from Kafka.
type ConsumerGroup struct {
	group   sarama.ConsumerGroup
	handler MessageHandler
	logger  *slog.Logger
}

// NewConsumerGroup creates a new Kafka consumer group.
func NewConsumerGroup(brokers, groupID string, logger *slog.Logger) (*ConsumerGroup, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	brokerList := strings.Split(brokers, ",")
	group, err := sarama.NewConsumerGroup(brokerList, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer group: %w", err)
	}

	return &ConsumerGroup{
		group:  group,
		logger: logger,
	}, nil
}

// Consume starts consuming messages from the specified topics.
// This is a blocking call and should be run in a goroutine.
func (c *ConsumerGroup) Consume(ctx context.Context, topics []string, handler MessageHandler) error {
	c.handler = handler

	for {
		if err := c.group.Consume(ctx, topics, c); err != nil {
			c.logger.Error("error from consumer", slog.String("error", err.Error()))
			return fmt.Errorf("consumer group error: %w", err)
		}
		// Check if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// Close shuts down the consumer group gracefully.
func (c *ConsumerGroup) Close() error {
	return c.group.Close()
}

// --- sarama.ConsumerGroupHandler interface ---

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (c *ConsumerGroup) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (c *ConsumerGroup) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from a partition claim.
func (c *ConsumerGroup) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := c.handler(session.Context(), msg); err != nil {
			c.logger.Error("failed to handle message",
				slog.String("topic", msg.Topic),
				slog.Int("partition", int(msg.Partition)),
				slog.Int64("offset", msg.Offset),
				slog.String("error", err.Error()),
			)
			continue
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
