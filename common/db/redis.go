package db

import (
	"context"

	"github.com/redis/go-redis/v9"
	"potat-api/common"
	"potat-api/common/logger"
)

type RedisClient struct {
	*redis.Client
}

var RedisErrNil = redis.Nil

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
