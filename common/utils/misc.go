package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandomString(length int) (string, error) {
	bytes := make([]byte, length / 2)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	hexKey := hex.EncodeToString(bytes)
	return hexKey, nil
}
