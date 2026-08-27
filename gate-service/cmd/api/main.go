package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entra-api/gate-service/internal/consumer"
	"entra-api/gate-service/internal/handler"
	"entra-api/gate-service/internal/repository/db"
	"entra-api/gate-service/internal/service"
	"entra-api/shared/config"
	sharedDb "entra-api/shared/database"
	"entra-api/shared/kafka"
	"entra-api/shared/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()

	// Override db name
	cfg.Database.DBName = getEnv("POSTGRES_DB_GATE", getEnv("GATE_DB", "entra_gate"))

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
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.Logger(logger))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	gateHandler := handler.NewGateHandler(gateService)
	handler.RegisterRoutes(router, gateHandler, cfg.JWT.Secret)

	port := getEnv("GATE_SERVICE_PORT", getEnv("PORT", "8086"))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Starting gate-service", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gate-service...")
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

