package db

import (
	"context"
	"fmt"
	"potat-api/common"
	"potat-api/common/utils"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	Clickhouse driver.Conn
)

func InitClickhouse(config common.Config) (error) {
	host := config.Clickhouse.Host
	if host == "" {
		host = "localhost"
	}

	port := config.Clickhouse.Port
	if port == "" {
		port = "9000"
	}

	hostStr := fmt.Sprintf("%s:%s", host, port)

	var (
		ctx = context.Background()
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr: []string{hostStr},
			Auth: clickhouse.Auth{
				Username: config.Clickhouse.User,
				Password: config.Clickhouse.Password,
			},
			Debugf: utils.Debug.Printf,
		})
	)

	if err != nil {
		return err
	}

	if err := conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			utils.Error.Panicf(
				"Exception [%d] %s \n%s\n", 
				exception.Code, 
				exception.Message, 
				exception.StackTrace,
			)
		}

		return err
	}

	Clickhouse = conn

	return nil
}