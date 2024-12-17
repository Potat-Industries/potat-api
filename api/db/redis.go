package db

import (
	"context"
	"potat-api/api/utils"

	"github.com/redis/go-redis/v9"
)

var (
	Redis *redis.Client
)

func InitRedis(config utils.Config) error {
	Redis = redis.NewClient(&redis.Options{
		Addr:     config.Redis.Host + ":" + config.Redis.Port,
		Password: "",
		DB:       0,
	})

	_, err := Redis.Ping(context.Background()).Result()
	if err != nil {
		return err
	}

	return nil
}