package database

import (
	"context"
	"fmt"
	"time"

	"entra-api/shared/config"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates and returns a new Redis client
func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password, // no password set
		DB:       0,            // use default DB
	})

	// Ping the Redis server to verify connection
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return client, nil
}
