package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entra-api/payment-service/internal/consumer"
	"entra-api/payment-service/internal/handler"
	"entra-api/payment-service/internal/repository/db"
	"entra-api/payment-service/internal/service"

	"entra-api/shared/config"
	"entra-api/shared/database"
	"entra-api/shared/kafka"
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	cfg.Database.DBName = getEnv("PAYMENT_DB", "entra_payment")
	cfg.Server.Port = getEnv("PAYMENT_SERVICE_PORT", "8084")

	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to db", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, logger)
	if err != nil {
		logger.Error("failed to init kafka producer", slog.String("error", err.Error()))
	} else {
		defer producer.Close()
	}

	queries := db.New(pool)
	paymentService := service.NewPaymentService(queries, producer)
	paymentHandler := handler.NewPaymentHandler(queries, paymentService)

	// Kafka Consumer
	orderConsumerGroup, err := kafka.NewConsumerGroup(cfg.Kafka.Brokers, "payment-service-group", logger)
	if err != nil {
		logger.Error("failed to create consumer group", slog.String("error", err.Error()))
	} else {
		defer orderConsumerGroup.Close()
		orderConsumerHandler := consumer.NewOrderConsumer(paymentService)
		
		go func() {
			topics := []string{"order.created", "order.cancelled"}
			if err := orderConsumerGroup.Consume(ctx, topics, orderConsumerHandler.HandleMessage); err != nil {
				logger.Error("consumer error", slog.String("error", err.Error()))
			}
		}()
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(logger))

	handler.RegisterRoutes(r, paymentHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("payment-service starting", slog.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
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
