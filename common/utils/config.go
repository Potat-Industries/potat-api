package utils

import (
	"encoding/json"
	"os"

	"potat-api/common"
)

var config *common.Config

func LoadConfig() *common.Config {
	if config != nil {
		return config
	}

	data, err := loadOrCopy()
	if err != nil {
		Error.Panicln("Failed loading config", err)
	}

	var configuration common.Config
	err = json.Unmarshal(data, &configuration)
	if err != nil {
		Error.Panicln("Failed unmarshaling config", err)
	}

	config = &configuration

	return config
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
	Warn.Println("Config file not found, copying example config")

	data, err := os.ReadFile("exampleconfig.json")
	if err != nil {
		return nil, err
	}

	err = os.Rename("exampleconfig.json", "config.json")
	if err != nil {
		return nil, err
	}

	return data, nil
}
