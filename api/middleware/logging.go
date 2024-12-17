package middleware

import (
	"time"
	"net/http"

	"potat-api/api/utils"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		loggingRW := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(loggingRW, r)

		utils.Debug.Printf(
			"%s %s | Status: %d | Duration: %v | User-Agent: %s",
			r.Method,
			r.RequestURI,
			loggingRW.statusCode,
			time.Since(startTime),
			r.UserAgent(),
		)
	})
}