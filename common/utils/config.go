// Package utils provides utility functions and types for all routes.
package utils

import (
	"encoding/json"
	"os"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
)

// LoadConfig loads the configuration from the config.json file.
func LoadConfig() *common.Config {
	data, err := loadOrCopy()
	if err != nil {
		logger.Error.Panicln("Failed loading config", err)
	}

	var configuration common.Config
	err = json.Unmarshal(data, &configuration)
	if err != nil {
		logger.Error.Panicln("Failed unmarshaling config", err)
	}

	return &configuration
}

func loadOrCopy() ([]byte, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		if os.IsNotExist(err) {
			return copyExampleConfig()
		}

		return nil, err
	}

	return data, nil
}

func copyExampleConfig() ([]byte, error) {
	logger.Warn.Println("Config file not found, copying example config")

	data, err := os.ReadFile("exampleconfig.json")
	if err != nil {
		return nil, err
	}

	err = os.WriteFile("config.json", data, 0o600)
	if err != nil {
		return nil, err
	}

	return data, nil
}
