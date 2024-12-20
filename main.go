package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"potat-api/api"
	"potat-api/haste"
	"potat-api/common/db"
	"potat-api/redirects"
	"potat-api/common/utils"

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

	err = db.Postgres.Ping(ctx)
	if err != nil {
		utils.Error.Panicln("Failed pinging Postgres", err)
	}
	utils.Info.Println("Postgres initialized")

	err = db.InitClickhouse(*config)
	if err != nil {
		utils.Error.Panicln("Failed initializing Clickhouse", err)
	} 

	err = db.Clickhouse.Ping(ctx)
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
	
	utils.Info.Println("Startup complete, serving APIs...")

	utils.CreateBroker(*config)

	apiChan := make(chan error)
	hastebinChan := make(chan error)
	// uploaderChan := make(chan error)
	redirectsChan := make(chan error)
	metricsChan := make(chan error)
	shutdownChan := make(chan os.Signal, 1)

	signal.Notify(
		shutdownChan, 
		os.Interrupt, 
		syscall.SIGTERM, 
		syscall.SIGINT,
	)

	go func() {
		hastebinChan <- haste.StartServing(*config)
	}()

	go func() {
		redirectsChan <- redirects.StartServing(*config)
	}()

	go func() {
		apiChan <- api.StartServing(*config)
	}()

	go func() {
		metricsChan <- utils.ObserveMetrics(*config)
	}()

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