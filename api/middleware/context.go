package middleware

import (
	"context"
	"errors"
	"net/http"

	"potat-api/common/db"
)

var ErrMissingContext = errors.New("missing database client in context")

type contextKey string

const (
	PostgresKey   contextKey = "postgres"
	RedisKey      contextKey = "redis"
	ClickhouseKey contextKey = "clickhouse"
)

// InjectDBMiddleware returns a middleware that injects DB clients into the request context.
func InjectDatabases(
	postgres 		*db.PostgresClient, 
	redis 			*db.RedisClient, 
	clickhouse 	*db.ClickhouseClient,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), PostgresKey, postgres)
			ctx = context.WithValue(ctx, RedisKey, redis)
			ctx = context.WithValue(ctx, ClickhouseKey, clickhouse)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
