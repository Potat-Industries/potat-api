// Package db provides database clients and functions to retrieve or update data.
package db

import (
	"context"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/redis/go-redis/v9"
)

// RedisClient is a wrapper around the Redis client to provide a custom client.
type RedisClient struct {
	*redis.Client
}

// ErrRedisNil is a constant for redis.Nil to handle nil responses from Redis.
var ErrRedisNil = redis.Nil

// InitRedis initializes a Redis client using the provided configuration.
func InitRedis(config common.Config) (*RedisClient, error) {
	host := config.Redis.Host
	if host == "" {
		host = "localhost"
	}

	port := config.Redis.Port
	if port == "" {
		port = "6379"
	}

	options := &redis.Options{
		Addr:     host + ":" + port,
		Password: "",
		DB:       0,
	}

	return &RedisClient{redis.NewClient(options)}, nil
}

// Scan retrieves keys from Redis that match a given pattern using the SCAN command.
func (r *RedisClient) Scan(
	ctx context.Context,
	match string,
	count int64,
	cursor uint64,
) ([]string, error) {
	matches := make([]string, 0)

	for cursor != 0 {
		keys, next, err := r.Client.Scan(ctx, cursor, match, count).Result()
		if err != nil {
			logger.Error.Println("Failed scanning keys", err)

			return nil, err
		}

		matches = append(matches, keys...)
		cursor = next
	}

	return matches, nil
}
