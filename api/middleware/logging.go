package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"potat-api/common/utils"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	headers    http.Header
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
	lrw.headers = lrw.ResponseWriter.Header()
}

func LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		loggingRW := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(loggingRW, r)

		cachehit := loggingRW.headers.Get("X-Cache-Hit")
		if cachehit == "" {
			cachehit = "MISS"
		}

		utils.ObserveInboundRequests(
			r.Host,
			r.RequestURI,
			r.RemoteAddr,
			r.Method,
			strconv.Itoa(loggingRW.statusCode),
			cachehit,
		)

		// Ignore chatterino link resolver xd
		agent := r.UserAgent()
		if strings.HasPrefix(agent, "chatterino-api-cache") {
			return
		}

		line := fmt.Sprintf(
			"Host: %s | %s %s | Cache %s | Status: %d | Duration: %v | User-Agent: %s",
		  	r.Host,
			r.Method,
			r.RequestURI,
			cachehit,
			loggingRW.statusCode,
			time.Since(startTime),
			agent,
		)

		if loggingRW.statusCode >= 500 {
			utils.Error.Println(line)
		} else if loggingRW.statusCode >= 400 && loggingRW.statusCode < 500 {
			utils.Warn.Println(line)
		} else {
			utils.Debug.Println(line)
		}
	})
}
