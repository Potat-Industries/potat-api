// Package db provides database clients and functions to retrieve or update data.
package db

import (
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
)

// ClickhouseClient is a wrapper around the ClickHouse driver.Conn to provide a custom client.
type ClickhouseClient struct {
	driver.Conn
}

// InitClickhouse initializes a ClickHouse connection using the provided configuration.
func InitClickhouse(config common.Config) (*ClickhouseClient, error) {
	host := config.Clickhouse.Host
	if host == "" {
		host = "localhost" //nolint:goconst
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
		Debugf: logger.Debug.Printf,
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	return &ClickhouseClient{conn}, nil
}
