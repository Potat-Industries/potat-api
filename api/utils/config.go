package utils

import (
	"encoding/json"
	"os"
)

type Config struct {
	Postgres   PostgresConfig   `json:"postgres"`
	Clickhouse ClickhouseConfig `json:"clickhouse"`
	Redis      RedisConfig      `json:"redis"`
}

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type ClickhouseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type RedisConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}