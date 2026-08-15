package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entra-api/shared/config"
	"entra-api/shared/database"
	"entra-api/shared/kafka"
	"entra-api/shared/middleware"
	
	"entra-api/ticket-service/internal/client"
	"entra-api/ticket-service/internal/consumer"
	"entra-api/ticket-service/internal/handler"
	"entra-api/ticket-service/internal/repository/db"
	"entra-api/ticket-service/internal/service"
	"entra-api/ticket-service/internal/worker"

	"github.com/gin-gonic/gin"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	cfg.Database.DBName = getEnv("POSTGRES_DB", "entra_event") // Using event DB since it holds the schema for testing or maybe separate DB. Let's use entra_ticket
	cfg.Database.DBName = getEnv("TICKET_DB", "entra_ticket")
	cfg.Server.Port = getEnv("TICKET_SERVICE_PORT", "8083")

	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to db", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	// Kafka Producer
	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, logger)
	if err != nil {
		logger.Error("failed to init kafka producer", slog.String("error", err.Error()))
		// In a real app we might fail, here we'll just log
	} else {
		defer producer.Close()
	}

	// Internal Clients
	eventServiceURL := getEnv("EVENT_SERVICE_URL", "http://localhost:8082")
	eventClient := client.NewEventClient(eventServiceURL)

	// Layers
	queries := db.New(pool)
	ticketService := service.NewTicketService(queries, eventClient, producer)
	orderHandler := handler.NewOrderHandler(ticketService)
	ticketHandler := handler.NewTicketHandler(ticketService)
	withdrawalHandler := handler.NewWithdrawalHandler(ticketService)

	// Kafka Consumer for Payments
	paymentConsumerGroup, err := kafka.NewConsumerGroup(cfg.Kafka.Brokers, "ticket-service-group", logger)
	if err != nil {
		logger.Error("failed to create consumer group", slog.String("error", err.Error()))
	} else {
		defer paymentConsumerGroup.Close()
		paymentConsumerHandler := consumer.NewPaymentConsumer(ticketService)

		go func() {
			topics := []string{"payment.success", "payment.failed"}
			if err := paymentConsumerGroup.Consume(ctx, topics, paymentConsumerHandler.HandleMessage); err != nil {
				logger.Error("payment consumer error", slog.String("error", err.Error()))
			}
		}()
	}

	// Kafka Consumer for General Events (Gate Scans)
	eventConsumerGroup, err := kafka.NewConsumerGroup(cfg.Kafka.Brokers, "ticket-event-group", logger)
	if err != nil {
		logger.Error("failed to create event consumer group", slog.String("error", err.Error()))
	} else {
		defer eventConsumerGroup.Close()
		eventConsumerHandler := consumer.NewEventConsumer(queries)

		go func() {
			topics := []string{"ticket.scanned"}
			if err := eventConsumerGroup.Consume(ctx, topics, eventConsumerHandler.HandleMessage); err != nil {
				logger.Error("event consumer error", slog.String("error", err.Error()))
			}
		}()
	}

	// Worker
	expiryWorker := worker.NewExpiryWorker(queries, ticketService)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go expiryWorker.Start(workerCtx)

	// Server
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logger))

	handler.RegisterRoutes(r, orderHandler, ticketHandler, withdrawalHandler, cfg.JWT.Secret)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("ticket-service starting", slog.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	workerCancel() // stop worker
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
