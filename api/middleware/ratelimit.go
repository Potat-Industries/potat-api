package middleware

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

var globalLimiter = rate.NewLimiter(
	rate.Every(100 * time.Second), 
	10,
)

func GlobalLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !globalLimiter.Allow() {
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

func SpecificLimiter(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
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