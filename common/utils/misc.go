// Package utils provides utility functions and types for all routes.
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// RandomString generates a random hexadecimal string of the specified length.
func RandomString(length int) (string, error) {
	bytes := make([]byte, length/2)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	hexKey := hex.EncodeToString(bytes)

	return hexKey, nil
}

// Humanize converts a time.Duration into a human-readable string format.
func Humanize(duration time.Duration, limit int) string {
	if duration == 0 {
		return "0s"
	}

	var result []string
	if duration.Hours() >= 8760 {
		years := int(duration.Hours() / 8760)
		result = append(result, fmt.Sprintf("%dy", years))
		duration -= time.Duration(years) * 8760 * time.Hour
	}

	if duration.Hours() >= 720 {
		months := int(duration.Hours() / 720)
		result = append(result, fmt.Sprintf("%dmo", months))
		duration -= time.Duration(months) * 720 * time.Hour
	}

	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		result = append(result, fmt.Sprintf("%dd", days))
		duration -= time.Duration(days) * 24 * time.Hour
	}

	if duration.Hours() >= 1 {
		hours := int(duration.Hours())
		result = append(result, fmt.Sprintf("%dh", hours))
		duration -= time.Duration(hours) * time.Hour
	}

	if duration.Minutes() >= 1 {
		minutes := int(duration.Minutes())
		result = append(result, fmt.Sprintf("%dm", minutes%60))
		duration -= time.Duration(minutes) * time.Minute
	}

	if duration.Seconds() >= 1 {
		seconds := int(duration.Seconds())
		result = append(result, fmt.Sprintf("%ds", seconds%60))
	}

	if limit > len(result) {
		limit = len(result)
	}

	return strings.Join(result[:limit], " ")
}
