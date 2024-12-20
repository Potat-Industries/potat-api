package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"potat-api/api"
	"potat-api/common/db"
	"potat-api/common/utils"
	"potat-api/haste"
	"potat-api/redirects"
	"potat-api/uploader"

	_ "potat-api/api/routes/get"
	_ "potat-api/api/routes/post"
)

func main() {
	utils.Info.Println("Starting Potat API...")

	ctx := context.Background()
	config, err := utils.LoadConfig("config.json")
	if err != nil {
		utils.Error.Panicln("Failed loading config", err)
	}

	err = db.InitPostgres(*config)
	if err != nil {
		utils.Error.Panicln("Failed initializing Postgres", err)
	} 

	err = runWithTimeout(db.Postgres.Ping, ctx)
	if err != nil {
		utils.Error.Panicln("Failed pinging Postgres", err)
	}
	utils.Info.Println("Postgres initialized")

	err = db.InitClickhouse(*config)
	if err != nil {
		utils.Error.Panicln("Failed initializing Clickhouse", err)
	} 

	err = runWithTimeout(db.Clickhouse.Ping, ctx)
	if err != nil {
		utils.Error.Panicln("Failed pinging Clickhouse", err)
	} 
	utils.Info.Println("Clickhouse initialized")

	err = db.InitRedis(*config)
	if err != nil {
		utils.Error.Panicln("Failed initializing Redis", err)
	}

	err = db.Redis.Ping(ctx).Err()
	if err != nil {
		utils.Error.Panicln("Failed pinging Redis", err)
	}
	utils.Info.Println("Redis initialized")

	if config.RabbitMQ.Enabled {
		cleanup, err := utils.CreateBroker(*config)
		if err != nil {
			utils.Error.Panicf("Failed to connect to RabbitMQ: %v", err)
		}
		defer cleanup()
	}

	utils.Info.Println("Startup complete, serving APIs...")

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(
		shutdownChan, 
		os.Interrupt, 
		syscall.SIGTERM, 
		syscall.SIGINT,
	)

	hastebinChan := make(chan error)
	if config.Haste.Enabled {
		go func() {
			hastebinChan <- haste.StartServing(*config)
		}()
	}

	redirectsChan := make(chan error)
	if config.Redirects.Enabled {
		go func() {
			redirectsChan <- redirects.StartServing(*config)
		}()
	}

	apiChan := make(chan error)
	if config.API.Enabled {
		go func() {
			apiChan <- api.StartServing(*config)
		}()
	}

	metricsChan := make(chan error)
	if config.Prometheus.Enabled {
		go func() {
			metricsChan <- utils.ObserveMetrics(*config)
		}()
	}

	uploaderChan := make(chan error)
	if config.Uploader.Enabled {
		go func() {
			uploaderChan <- uploader.StartServing(*config)
		}()
	}

	select {
	case err := <-hastebinChan:
		utils.Error.Panicln("Hastebin server error!", err)
	case err := <-redirectsChan:
		utils.Error.Panicln("Redirects server error!", err)
	case err := <-apiChan:
		utils.Error.Panicln("API server error!", err)
	case err := <-metricsChan:
		utils.Error.Panicln("Metrics server error!", err)
	case <-shutdownChan:
		utils.Warn.Println("Shutdown requested...")
	}

	// Do something i guess

	utils.Warn.Println("Shutting down...")
}

func runWithTimeout(
	f func(ctx context.Context) error, 
	ctx context.Context,
	) error {
	done := make(chan error, 1)

	var lastError error
	for i := 0; i < 3; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 1 * time.Second)
		defer cancel()

		go func() {	done <- f(attemptCtx) }()

		select {
		case err := <-done:
			return err
		case <-attemptCtx.Done():
			utils.Warn.Println("Ping timed out, retrying...")
			lastError = errors.New("ping timed out")
		}
	}

	return lastError
}