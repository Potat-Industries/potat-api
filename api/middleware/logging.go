package middleware

import (
	"time"
	"strings"
	"strconv"
	"net/http"

	"potat-api/common/utils"
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

		utils.ObserveInboundRequests(
			r.Host, 
			r.RequestURI, 
			r.RemoteAddr,
			r.Method, 
			strconv.Itoa(loggingRW.statusCode), 
		)

		utils.Debug.Printf(
			"Port: %s | %s %s | Status: %d | Duration: %v | User-Agent: %s",
		  strings.Split(r.Host, ":")[1],
			r.Method,
			r.RequestURI,
			loggingRW.statusCode,
			time.Since(startTime),
			r.UserAgent(),
		)
	})
}