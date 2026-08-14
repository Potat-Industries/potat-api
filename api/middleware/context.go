package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/utils"
)

var ErrMissingContext = errors.New("missing database client in context")

type contextKey string

//nolint:revive
const (
	PostgresKey      contextKey = "postgres"
	RedisKey         contextKey = "redis"
	ClickhouseKey    contextKey = "clickhouse"
	NatsKey          contextKey = "nats"
	AuthenticatorKey contextKey = "authenticator"
)

func InjectDatabases(
	postgres *db.PostgresClient,
	redis *db.RedisClient,
	clickhouse *db.ClickhouseClient,
	nats *utils.NatsClient,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), PostgresKey, postgres)
			ctx = context.WithValue(ctx, RedisKey, redis)
			ctx = context.WithValue(ctx, ClickhouseKey, clickhouse)
			ctx = context.WithValue(ctx, NatsKey, nats)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
