package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"potat-api/common/db"
	"potat-api/common/utils"
)

var errBadRedisResponse = errors.New("invalid result from Redis")

// NewRateLimiter returns a new rate limiter middleware.
func NewRateLimiter(limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			ip := request.RemoteAddr
			if forwardedFor := request.Header.Get("CF-Connecting-IP"); forwardedFor != "" {
				ip = forwardedFor
			}

			allowed, remaining, remainingTTL, err := getIPToken(request.Context(), ip, limit, window)
			if err != nil {
				http.Error(
					writer,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)

				return
			}

			writer.Header().Set("X-RateLimit-Reset", strconv.FormatInt(remainingTTL, 10))
			writer.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
			writer.Header().Set("X-RateLimit-Window", strconv.FormatInt(int64(window.Seconds()), 10))
			writer.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

			if !allowed {
				writer.Header().Set("Retry-After", strconv.FormatInt(remainingTTL, 10))

				http.Error(
					writer,
					http.StatusText(http.StatusTooManyRequests),
					http.StatusTooManyRequests,
				)

				return
			}

			next.ServeHTTP(writer, request)
		})
	}
}

func getIPToken(
	ctx context.Context,
	ip string,
	limit int64,
	window time.Duration,
) (bool, int64, int64, error) {
	// Set key expiry if its first request from an ip
	luaScript := `
		local current = redis.call("INCR", KEYS[1])
		if current == 1 then
				redis.call("EXPIRE", KEYS[1], ARGV[1])
		end

		local ttl = redis.call("TTL", KEYS[1])
		local allowed = 0
		if current <= tonumber(ARGV[2]) then
			allowed = 1
		end

		return {current, allowed, ttl}
	`

	result, err := db.Redis.Eval(
		ctx,
		luaScript,
		[]string{ip},
		int(window.Seconds()),
		limit,
	).Result()
	if err != nil {
		utils.Error.Println("Error evaluating Lua script", err)

		return false, 0, 0, err
	}

	results, ok := result.([]interface{})
	if !ok {
		return false, 0, 0, errBadRedisResponse
	}
	if len(results) != 3 {
		return false, 0, 0, errBadRedisResponse
	}

	remainder, ok := results[0].(int64)
	if !ok {
		return false, 0, 0, errBadRedisResponse
	}
	remaining := limit - remainder

	allowedInt, ok := results[1].(int64)
	if !ok {
		return false, 0, 0, errBadRedisResponse
	}
	allowed := allowedInt == 1

	remainingTTL, ok := results[2].(int64)
	if !ok {
		return false, 0, 0, errBadRedisResponse
	}

	return allowed, remaining, remainingTTL, nil
}
