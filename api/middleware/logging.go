package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/Potat-Industries/potat-api/common/utils"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	headers    http.Header
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
	lrw.headers = lrw.ResponseWriter.Header()
}

// LogRequest logs the request method, URI, status code, and duration of the request.
func LogRequest(metrics *utils.Metrics) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			startTime := time.Now()

			loggingRW := &loggingResponseWriter{ResponseWriter: writer, statusCode: http.StatusOK}

			next.ServeHTTP(loggingRW, request)

			cachehit := loggingRW.headers.Get("X-Cache-Hit")
			if cachehit == "" {
				cachehit = "MISS"
			}

			metrics.ObserveInboundRequests(
				request.Host,
				request.RequestURI,
				request.RemoteAddr,
				request.Method,
				strconv.Itoa(loggingRW.statusCode),
				cachehit,
			)

			// Ignore chatterino link resolver xd
			agent := request.UserAgent()
			if strings.HasPrefix(agent, "chatterino-api-cache") {
				return
			}

			line := fmt.Sprintf(
				"Host: %s | %s %s | Cache %s | Status: %d | Duration: %v | User-Agent: %s",
				request.Host,
				request.Method,
				request.RequestURI,
				cachehit,
				loggingRW.statusCode,
				time.Since(startTime),
				agent,
			)

			switch {
			case loggingRW.statusCode >= 500:
				logger.Error.Println(line)
			case loggingRW.statusCode >= 400 && loggingRW.statusCode < 500:
				logger.Warn.Println(line)
			default:
				logger.Debug.Println(line)
			}
		})
	}
}
