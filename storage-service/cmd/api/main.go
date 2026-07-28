package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"context"

	"entra-api/storage-service/internal/handler"
	"entra-api/shared/config"
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()
	cfg.Server.Port = getEnv("STORAGE_SERVICE_PORT", "8087")

	minioEndpoint := getEnv("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := getEnv("MINIO_ROOT_USER", "entra_minio")
	minioSecretKey := getEnv("MINIO_ROOT_PASSWORD", "entra_minio_secret")
	useSSL := false

	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		logger.Error("failed to connect to minio", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("connected to MinIO")

	bucketName := "entra-media"

	// Check bucket exists
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		logger.Error("failed to check minio bucket", slog.String("error", err.Error()))
	} else if !exists {
		logger.Warn("bucket does not exist yet", slog.String("bucket", bucketName))
	}

	storageHandler := handler.NewStorageHandler(minioClient, bucketName, minioEndpoint)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logger))

	handler.RegisterRoutes(r, storageHandler, cfg.JWT.Secret)

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("storage-service starting", slog.String("port", cfg.Server.Port))
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

	logger.Info("storage-service stopped")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
