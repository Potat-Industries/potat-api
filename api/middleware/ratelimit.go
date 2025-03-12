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

func NewRateLimiter(limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if forwardedFor := r.Header.Get("CF-Connecting-IP"); forwardedFor != "" {
				ip = forwardedFor
			}

			allowed, remaining, remainingTTL, err := getIpToken(ip, r.Context(), limit, window)
			if err != nil {
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)

				return
			}

			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(remainingTTL, 10))
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
			w.Header().Set("X-RateLimit-Window", strconv.FormatInt(int64(window.Seconds()), 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

			if !allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(remainingTTL, 10))

				http.Error(
					w,
					http.StatusText(http.StatusTooManyRequests),
					http.StatusTooManyRequests,
				)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getIpToken(
	ip string,
	ctx context.Context,
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

	results := result.([]interface{})
	if len(results) != 3 {
		return false, 0, 0, errors.New("invalid result from Redis")
	}

	remaining := limit - results[0].(int64)
	allowed := results[1].(int64) == 1
	remainingTTL := results[2].(int64)

	return allowed, remaining, remainingTTL, nil
}
