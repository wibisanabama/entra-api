package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entra-api/cashless-service/internal/consumer"
	"entra-api/cashless-service/internal/handler"
	"entra-api/cashless-service/internal/repository/db"
	"entra-api/cashless-service/internal/service"

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
	cfg.Database.DBName = getEnv("CASHLESS_DB", "entra_cashless")
	cfg.Server.Port = getEnv("CASHLESS_SERVICE_PORT", "8085")

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
	walletService := service.NewWalletService(queries, producer)
	walletHandler := handler.NewWalletHandler(walletService)

	// Kafka Consumer
	paymentConsumerGroup, err := kafka.NewConsumerGroup(cfg.Kafka.Brokers, "cashless-service-group", logger)
	if err != nil {
		logger.Error("failed to create consumer group", slog.String("error", err.Error()))
	} else {
		defer paymentConsumerGroup.Close()
		paymentConsumerHandler := consumer.NewPaymentConsumer(walletService)
		
		go func() {
			topics := []string{"payment.success", "payment.failed"}
			if err := paymentConsumerGroup.Consume(ctx, topics, paymentConsumerHandler.HandleMessage); err != nil {
				logger.Error("consumer error", slog.String("error", err.Error()))
			}
		}()
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(logger))

	handler.RegisterRoutes(r, walletHandler, cfg.JWT.Secret)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("cashless-service starting", slog.String("port", cfg.Server.Port))
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
