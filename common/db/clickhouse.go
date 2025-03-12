package db

import (
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"potat-api/common"
	"potat-api/common/utils"
)

var Clickhouse driver.Conn

func InitClickhouse(config common.Config) error {
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

	options := &clickhouse.Options{
		Addr:   []string{fmt.Sprintf("%s:%s", host, port)},
		Auth:   clickhouse.Auth{Username: user, Password: config.Clickhouse.Password},
		Debugf: utils.Debug.Printf,
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return err
	}

	Clickhouse = conn

	return nil
}
