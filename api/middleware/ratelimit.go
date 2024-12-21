package middleware

import (
	"time"
	"strconv"
	"context"
	"net/http"

	"potat-api/common/db"
)


// TODO make this configurable per route or something

func GlobalLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwardedFor := r.Header.Get("CF-Connecting-IP"); forwardedFor != "" {
			ip = forwardedFor
		}

		allowed, remaining, remainingTTL, err := getIpToken(
			ip,
			r.Context(),
			100,
			1 * time.Minute,
		)

		if err != nil {
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
		}

		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(remainingTTL, 10))
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if !allowed {
			http.Error(
				w,
				http.StatusText(http.StatusTooManyRequests),
				http.StatusTooManyRequests,
			)
		}
		
		w.Header().Set("X-Forwarded-For", ip)

		next.ServeHTTP(w, r)
	})
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
		local allowed = current <= tonumber(ARGV[2])

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
		return false, 0, 0, err
	}

	results := result.([]interface{})
	remaining := limit - results[0].(int64)
	allowed := results[1].(int64) == 1
	remainingTTL := results[2].(int64)  

	return allowed, remaining, remainingTTL, nil
}