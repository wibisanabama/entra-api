package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"net/http"

	"entra-api/auth-service/internal/handler"
	"entra-api/auth-service/internal/repository/db"
	"entra-api/auth-service/internal/service"
	"entra-api/shared/config"
	"entra-api/shared/database"
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load config
	cfg := config.Load()

	// Override DB name for auth service
	cfg.Database.DBName = getEnv("POSTGRES_DB", "entra_auth")
	cfg.Server.Port = getEnv("AUTH_SERVICE_PORT", "8081")

	// Connect to PostgreSQL
	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	// Initialize layers
	queries := db.New(pool)
	authService := service.NewAuthService(queries, cfg)
	authHandler := handler.NewAuthHandler(authService)

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logger))

	// Register routes
	handler.RegisterRoutes(r, authHandler, cfg.JWT.Secret)

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("auth-service starting", slog.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced shutdown", slog.String("error", err.Error()))
	}

	logger.Info("auth-service stopped")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
