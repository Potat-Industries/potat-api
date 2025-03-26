// Package redirects provides a public api to create short redirect urls.
package redirects

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/logger"
	"potat-api/common/utils"
)

const createTable = `
	CREATE TABLE IF NOT EXISTS url_redirects (
		key VARCHAR(9) PRIMARY KEY,
		url VARCHAR(500) NOT NULL
	);
`

type redirects struct {
	server   *http.Server
	postgres *db.PostgresClient
	redis    *db.RedisClient
}

// StartServing will start the redirects server on the configured port.
func StartServing(
	ctx context.Context,
	config common.Config,
	postgres *db.PostgresClient,
	redis *db.RedisClient,
	metrics *utils.Metrics,
) error {
	if config.Redirects.Host == "" || config.Redirects.Port == "" {
		logger.Error.Fatal("Config: Redirect host and port must be set")
	}

	redirector := redirects{
		postgres: postgres,
		redis:    redis,
	}

	router := mux.NewRouter()

	limiter := middleware.NewRateLimiter(100, 1*time.Minute, redis)
	router.Use(middleware.LogRequest(metrics))
	router.Use(limiter)
	router.HandleFunc("/{id}", redirector.getRedirect).Methods(http.MethodGet)

	redirector.server = &http.Server{
		Handler:      router,
		Addr:         config.Redirects.Host + ":" + config.Redirects.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	redirector.postgres.CheckTableExists(ctx, createTable)

	logger.Info.Printf("Redirects listening on %s", redirector.server.Addr)

	return redirector.server.ListenAndServe()
}

func (r *redirects) setRedis(ctx context.Context, key, data string) {
	err := r.redis.SetEx(ctx, key, data, time.Hour).Err()
	if err != nil {
		logger.Error.Printf("Error caching redirect: %v", err)
	}
}

func (r *redirects) getRedis(ctx context.Context, key string) (string, error) {
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, db.ErrRedisNil) {
		return "", err
	}

	return data, nil
}

func (r *redirects) getRedirect(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	key := vars["id"]
	if key == "" {
		http.NotFound(writer, request)

		return
	}

	cache, err := r.getRedis(request.Context(), key)
	if err == nil && cache != "" {
		writer.Header().Set("X-Cache-Hit", "HIT")
		http.Redirect(writer, request, cache, http.StatusSeeOther)

		return
	}
	writer.Header().Set("X-Cache-Hit", "MISS")

	redirect, err := r.postgres.GetRedirectByKey(request.Context(), key)
	if err != nil {
		if errors.Is(err, db.ErrPostgresNoRows) {
			http.NotFound(writer, request)

			return
		}

		logger.Error.Printf("Error fetching redirect: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if !strings.HasPrefix(redirect, "https://") {
		redirect = "https://" + redirect
	}

	go r.setRedis(request.Context(), key, redirect)

	request.Header.Set("X-Cache-Hit", "MISS")
	http.Redirect(writer, request, redirect, http.StatusSeeOther)
}
