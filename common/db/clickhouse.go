package db

import (
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

	user := config.Clickhouse.User
	if user == "" {
		user = "default"
	}

	hostStr := fmt.Sprintf("%s:%s", host, port)

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{hostStr},
		Auth: clickhouse.Auth{Username: user,	Password: config.Clickhouse.Password},
		Debugf: utils.Debug.Printf,
	})

	if err != nil {
		return err
	}

	Clickhouse = conn

	return nil
}