package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entra-api/event-service/internal/handler"
	"entra-api/event-service/internal/repository/db"
	"entra-api/event-service/internal/service"
	"entra-api/shared/config"
	"entra-api/shared/database"
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()
	cfg.Database.DBName = getEnv("POSTGRES_DB", "entra_event")
	cfg.Server.Port = getEnv("EVENT_SERVICE_PORT", "8082")

	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	// Setup Redis
	redisClient, err := database.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		logger.Error("failed to connect to redis", slog.String("error", err.Error()))
		// We can still run without cache, but for this exercise we'll exit or log. Let's just log and continue, or exit.
		// Exiting is safer to guarantee cache behavior.
		os.Exit(1)
	}
	defer redisClient.Close()
	logger.Info("connected to Redis")

	// Initialize layers
	queries := db.New(pool)
	venueService := service.NewVenueService(queries)
	venueHandler := handler.NewVenueHandler(venueService)
	eventService := service.NewEventService(queries, redisClient)
	eventHandler := handler.NewEventHandler(eventService)
	categoryHandler := handler.NewCategoryHandler(queries)
	internalTicketHandler := handler.NewInternalTicketHandler(queries)
	ticketTypeHandler := handler.NewTicketTypeHandler(eventService)

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logger))

	// Register routes
	handler.RegisterRoutes(r, eventHandler, venueHandler, categoryHandler, internalTicketHandler, ticketTypeHandler, cfg.JWT.Secret)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("event-service starting", slog.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced shutdown", slog.String("error", err.Error()))
	}

	logger.Info("event-service stopped")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
