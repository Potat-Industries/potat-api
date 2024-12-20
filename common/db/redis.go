package db

import (
	"context"
	"potat-api/common"

	"github.com/redis/go-redis/v9"
)

var (
	Redis *redis.Client
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

	_, err := Redis.Ping(context.Background()).Result()
	if err != nil {
		return err
	}

	return nil
}