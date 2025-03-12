package db

import (
	"context"
	"potat-api/common"
	"potat-api/common/utils"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	*redis.Client
}

var (
	Redis       *redis.Client
	RedisErrNil = redis.Nil
)

func InitRedis(config common.Config) error {
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

	Redis = redis.NewClient(options)

	return nil
}

func Scan(
	ctx context.Context,
	match string,
	count int64,
	cursor uint64,
) ([]string, error) {
	matches := make([]string, 0)

	for cursor != 0 {
		keys, next, err := Redis.Scan(ctx, cursor, match, count).Result()
		if err != nil {
			utils.Error.Println("Failed scanning keys", err)
			return nil, err
		}

		matches = append(matches, keys...)
		cursor = next
	}

	return matches, nil
}
