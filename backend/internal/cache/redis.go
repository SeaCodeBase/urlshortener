// backend/internal/cache/redis.go
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info(ctx, "Connected to Redis")
	return client, nil
}
