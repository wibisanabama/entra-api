package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/IBM/sarama"
)

// Producer wraps a Sarama SyncProducer for publishing messages to Kafka.
type Producer struct {
	producer sarama.SyncProducer
	logger   *slog.Logger
}

// NewProducer creates a new Kafka producer.
func NewProducer(brokers string, logger *slog.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 3
	config.Producer.Return.Successes = true

	brokerList := strings.Split(brokers, ",")
	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	return &Producer{
		producer: producer,
		logger:   logger,
	}, nil
}

// Publish sends a message to the specified Kafka topic.
func (p *Producer) Publish(_ context.Context, topic string, key, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	p.logger.Debug("message published",
		slog.String("topic", topic),
		slog.Int("partition", int(partition)),
		slog.Int64("offset", offset),
	)

	return nil
}

// Close shuts down the producer gracefully.
func (p *Producer) Close() error {
	return p.producer.Close()
}
