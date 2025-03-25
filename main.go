package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"potat-api/api"
	_ "potat-api/api/routes/get"
	_ "potat-api/api/routes/post"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/logger"
	"potat-api/common/utils"
	"potat-api/haste"
	"potat-api/redirects"
	"potat-api/socket"
	"potat-api/uploader"
)

func main() {
	logger.Info.Println("Starting Potat API...")

	ctx, cancel := context.WithCancel(context.Background())
	config := utils.LoadConfig()

	postgres := initPostgres(*config, ctx)

	redis := initRedis(*config, ctx)

	var clickhouse *db.ClickhouseClient
	if config.API.Enabled || config.Loops.Enabled {
		clickhouse = initClickhouse(*config, ctx)
	}

	var nats *utils.NatsClient
	if config.Nats.Enabled {
		nats = initNats(ctx)
		defer nats.Client.Drain()
	}

	if config.Loops.Enabled {
		go db.StartLoops(ctx, *config, nats, postgres, clickhouse, redis)
	}

	logger.Info.Println("Startup complete, serving APIs...")

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(
		shutdownChan,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
	)

	var metrics *utils.Metrics
	metricsChan := make(chan error)
	if config.Prometheus.Enabled {
		go func() {
			var metricErr error
			metrics, metricErr = utils.ObserveMetrics(*config)
			metricsChan <- metricErr
		}()
	}

	socketChan := make(chan error)
	if config.Socket.Enabled {
		go func() {
			socketChan <- socket.StartServing(*config, nats, metrics)
		}()
	}

	hastebinChan := make(chan error)
	if config.Haste.Enabled {
		go func() {
			hastebinChan <- haste.StartServing(*config, postgres, redis, metrics)
		}()
	}

	redirectsChan := make(chan error)
	if config.Redirects.Enabled {
		go func() {
			redirectsChan <- redirects.StartServing(*config, postgres, redis, metrics)
		}()
	}

	apiChan := make(chan error)
	if config.API.Enabled {
		go func() {
			apiChan <- api.StartServing(*config, postgres, redis, clickhouse, metrics)
		}()
	}

	uploaderChan := make(chan error)
	if config.Uploader.Enabled {
		go func() {
			uploaderChan <- uploader.StartServing(*config, postgres, redis, metrics)
		}()
	}

	select {
	case err := <-hastebinChan:
		logger.Error.Panicln("Hastebin server error! ", err)
	case err := <-redirectsChan:
		logger.Error.Panicln("Redirects server error! ", err)
	case err := <-apiChan:
		logger.Error.Panicln("API server error! ", err)
	case err := <-metricsChan:
		logger.Error.Panicln("Metrics server error! ", err)
	case err := <-uploaderChan:
		logger.Error.Panicln("Uploader server error! ", err)
	case err := <-socketChan:
		logger.Error.Panicln("Socket server error! ", err)
	case <-shutdownChan:
		logger.Warn.Println("Shutdown requested...")
	}

	if config.API.Enabled {
		clickhouse.Close()
		logger.Warn.Println("Clickhouse connection closed")
	}

	postgres.Close()
	logger.Warn.Println("Postgres connection closed")
	redis.Close()
	logger.Warn.Println("Redis connection closed")

	cancel()
}

func runWithTimeout(
	f func(ctx context.Context) error,
	ctx context.Context,
) error {
	done := make(chan error, 1)

	var lastError error
	for i := 0; i < 3; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		go func() { done <- f(attemptCtx) }()

		select {
		case err := <-done:
			return err
		case <-attemptCtx.Done():
			logger.Warn.Println("Ping timed out, retrying...")
			lastError = errors.New("ping timed out")
		}
	}

	return lastError
}

func initPostgres(config common.Config, ctx context.Context) *db.PostgresClient {
	postgres, err := db.InitPostgres(config)
	if err != nil {
		logger.Error.Panicln("Failed initializing Postgres", err)
	}
	err = runWithTimeout(postgres.Ping, ctx)
	if err != nil {
		logger.Error.Panicln("Failed pinging Postgres", err)
	}
	logger.Info.Println("Postgres initialized")

	return postgres
}

func initRedis(config common.Config, ctx context.Context) *db.RedisClient {
	redis, err := db.InitRedis(config)
	if err != nil {
		logger.Error.Panicln("Failed initializing Redis", err)
	}

	err = redis.Ping(ctx).Err()
	if err != nil {
		logger.Error.Panicln("Failed pinging Redis", err)
	}
	logger.Info.Println("Redis initialized")

	return redis
}

func initClickhouse(config common.Config, ctx context.Context) *db.ClickhouseClient {
	ch, err := db.InitClickhouse(config)
	if err != nil {
		logger.Error.Panicln("Failed initializing Clickhouse", err)
	}

	err = runWithTimeout(ch.Ping, ctx)
	if err != nil {
		logger.Error.Panicln("Failed pinging Clickhouse", err)
	}
	logger.Info.Println("Clickhouse initialized")

	return ch
}

func initNats(ctx context.Context) *utils.NatsClient {
	nats, err := utils.CreateNatsBroker(ctx)
	if err != nil {
		logger.Error.Panicf("Failed to connect to RabbitMQ: %v", err)
	}

	return nats
}
