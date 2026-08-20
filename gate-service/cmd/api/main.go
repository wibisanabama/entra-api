package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"entra-api/gate-service/internal/consumer"
	"entra-api/gate-service/internal/handler"
	"entra-api/gate-service/internal/repository/db"
	"entra-api/gate-service/internal/service"
	"entra-api/shared/config"
	sharedDb "entra-api/shared/database"
	"entra-api/shared/kafka"
	"github.com/gin-gonic/gin"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()

	// Override db name
	dbName := os.Getenv("GATE_DB")
	if dbName == "" {
		dbName = "entra_gate"
	}
	cfg.Database.DBName = dbName

	ctx := context.Background()

	// Setup Database
	pool, err := sharedDb.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	queries := db.New(pool)

	// Setup Kafka Producer
	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, logger)
	if err != nil {
		slog.Error("Failed to create Kafka producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	// Setup Service
	gateService := service.NewGateService(queries, producer)

	// Setup Kafka Consumer
	ticketConsumer := consumer.NewTicketConsumer(gateService)
	consumerGroup, err := kafka.NewConsumerGroup(cfg.Kafka.Brokers, "gate-service-group", logger)
	if err != nil {
		slog.Error("Failed to create Kafka consumer group", "error", err)
		os.Exit(1)
	}
	defer consumerGroup.Close()
	
	go func() {
		err := consumerGroup.Consume(ctx, []string{"ticket.created"}, ticketConsumer.HandleMessage)
		if err != nil {
			slog.Error("Failed to start Kafka consumer", "error", err)
		}
	}()

	// Setup Gin Server
	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	gateHandler := handler.NewGateHandler(gateService)
	handler.RegisterRoutes(router, gateHandler, cfg.JWT.Secret)

	port := os.Getenv("GATE_SERVICE_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8086"
	}

	slog.Info("Starting gate-service", "port", port)
	if err := router.Run(fmt.Sprintf(":%s", port)); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
