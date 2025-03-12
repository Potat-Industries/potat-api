package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func RandomString(length int) (string, error) {
	bytes := make([]byte, length/2)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	hexKey := hex.EncodeToString(bytes)

	return hexKey, nil
}

func Humanize(d time.Duration, limit int) string {
	if d == 0 {
		return "0s"
	}

	var result []string
	if d.Hours() >= 8760 {
		years := int(d.Hours() / 8760)
		result = append(result, fmt.Sprintf("%dy", years))
		d -= time.Duration(years) * 8760 * time.Hour
	}

	if d.Hours() >= 720 {
		months := int(d.Hours() / 720)
		result = append(result, fmt.Sprintf("%dmo", months))
		d -= time.Duration(months) * 720 * time.Hour
	}

	if d.Hours() >= 24 {
		days := int(d.Hours() / 24)
		result = append(result, fmt.Sprintf("%dd", days))
		d -= time.Duration(days) * 24 * time.Hour
	}

	if d.Hours() >= 1 {
		hours := int(d.Hours())
		result = append(result, fmt.Sprintf("%dh", hours))
		d -= time.Duration(hours) * time.Hour
	}

	if d.Minutes() >= 1 {
		minutes := int(d.Minutes())
		result = append(result, fmt.Sprintf("%dm", minutes%60))
		d -= time.Duration(minutes) * time.Minute
	}

	if d.Seconds() >= 1 {
		seconds := int(d.Seconds())
		result = append(result, fmt.Sprintf("%ds", seconds%60))
	}

	if limit > len(result) {
		limit = len(result)
	}

	return strings.Join(result[:limit], " ")
}
