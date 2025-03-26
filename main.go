// Project: potat-api
// Package main initializes the potat-api server and starts all the necessary components,
// including Postgres, Redis, Clickhouse, NATS, and various API services.
//
// It also handles graceful shutdowns, flushing buffers before exiting.
package main

import (
	"context"
	"errors"
	"net/http"
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

var errPingTimeout = errors.New("ping timed out")

func main() { //nolint:cyclop
	logger.Info.Println("Starting Potat API...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := utils.LoadConfig()

	postgres := initPostgres(ctx, *config)
	redis := initRedis(ctx, *config)
	clickhouse := initClickhouse(ctx, *config)

	var nats *utils.NatsClient
	if config.Nats.Enabled {
		nats = initNats(ctx)
		defer func() {
			if err := nats.Client.Drain(); err != nil {
				logger.Error.Panicln("Failed closing NATS connection", err)
			}
			logger.Warn.Println("NATS connection closed")
		}()
	}

	go db.StartLoops(ctx, *config, nats, postgres, clickhouse, redis)

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
		var server *http.Server
		metrics, server = utils.ObserveMetrics(*config)
		go func() {
			metricsChan <- server.ListenAndServe()
		}()
	} else {
		metrics = &utils.Metrics{}
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
			hastebinChan <- haste.StartServing(ctx, *config, postgres, redis, metrics)
		}()
	}

	redirectsChan := make(chan error)
	if config.Redirects.Enabled {
		go func() {
			redirectsChan <- redirects.StartServing(ctx, *config, postgres, redis, metrics)
		}()
	}

	uploaderChan := make(chan error)
	if config.Uploader.Enabled {
		go func() {
			uploaderChan <- uploader.StartServing(ctx, *config, postgres, redis, metrics)
		}()
	}

	apiChan := make(chan error)
	if config.API.Enabled {
		go func() {
			apiChan <- api.StartServing(*config, postgres, redis, clickhouse, metrics)
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
		if err := clickhouse.Close(); err != nil {
			logger.Error.Panicln("Failed closing Clickhouse connection", err)
		}
		logger.Warn.Println("Clickhouse connection closed")
	}

	if err := redis.Close(); err != nil {
		logger.Error.Panicln("Failed closing Redis connection", err)
	}
	logger.Warn.Println("Redis connection closed")

	postgres.Close()
	logger.Warn.Println("Postgres connection closed")
}

func runWithTimeout(
	ctx context.Context,
	f func(ctx context.Context) error,
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
			lastError = errPingTimeout
		}
	}

	return lastError
}

func initPostgres(ctx context.Context, config common.Config) *db.PostgresClient {
	postgres, err := db.InitPostgres(ctx, config)
	if err != nil {
		logger.Error.Panicln("Failed initializing Postgres", err)
	}
	err = runWithTimeout(ctx, postgres.Ping)
	if err != nil {
		logger.Error.Panicln("Failed pinging Postgres", err)
	}
	logger.Info.Println("Postgres initialized")

	return postgres
}

func initRedis(ctx context.Context, config common.Config) *db.RedisClient {
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

func initClickhouse(ctx context.Context, config common.Config) *db.ClickhouseClient {
	if !config.API.Enabled && !config.Loops.Enabled {
		return nil
	}

	ch, err := db.InitClickhouse(config)
	if err != nil {
		logger.Error.Panicln("Failed initializing Clickhouse", err)
	}

	err = runWithTimeout(ctx, ch.Ping)
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
