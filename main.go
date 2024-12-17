package main

import (
	"context"

	"potat-api/api"
	"potat-api/api/db"
	"potat-api/api/utils"

	_ "potat-api/api/routes/get"
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

	api.StartServing()
}