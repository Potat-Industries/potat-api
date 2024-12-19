package utils

import (
	"encoding/json"
	"os"

	"potat-api/common"
)

func LoadConfig(path string) (*common.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config common.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}