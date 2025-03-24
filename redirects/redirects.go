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
	"potat-api/common/utils"
)

const createTable = `
	CREATE TABLE IF NOT EXISTS url_redirects (
		key VARCHAR(9) PRIMARY KEY,
		url VARCHAR(500) NOT NULL
	);
`

type redirects struct {
	server *http.Server
}

// StartServing will start the redirects server on the configured port.
func StartServing(config common.Config) error {
	if config.Redirects.Host == "" || config.Redirects.Port == "" {
		utils.Error.Fatal("Config: Redirect host and port must be set")
	}

	redirector := redirects{}

	router := mux.NewRouter()

	limiter := middleware.NewRateLimiter(100, 1*time.Minute)
	router.Use(middleware.LogRequest)
	router.Use(limiter)
	router.HandleFunc("/{id}", redirector.getRedirect).Methods(http.MethodGet)

	redirector.server = &http.Server{
		Handler:      router,
		Addr:         config.Redirects.Host + ":" + config.Redirects.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	db.Postgres.CheckTableExists(createTable)

	utils.Info.Printf("Redirects listening on %s", redirector.server.Addr)

	return redirector.server.ListenAndServe()
}

func (r *redirects) setRedis(ctx context.Context, key, data string) {
	err := db.Redis.SetEx(ctx, key, data, time.Hour).Err()
	if err != nil {
		utils.Error.Printf("Error caching redirect: %v", err)
	}
}

func (r *redirects) getRedis(ctx context.Context, key string) (string, error) {
	data, err := db.Redis.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, db.RedisErrNil) {
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

	redirect, err := db.Postgres.GetRedirectByKey(request.Context(), key)
	if err != nil {
		if errors.Is(err, db.PostgresNoRows) {
			http.NotFound(writer, request)

			return
		}

		utils.Error.Printf("Error fetching redirect: %v", err)
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
