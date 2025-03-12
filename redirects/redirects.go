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

var (
	server *http.Server
	router *mux.Router
)

func init() {
	router = mux.NewRouter()

	limiter := middleware.NewRateLimiter(100, 1*time.Minute)
	router.Use(middleware.LogRequest)
	router.Use(limiter)
	router.HandleFunc("/{id}", getRedirect).Methods(http.MethodGet)
}

func StartServing(config common.Config) error {
	if config.Redirects.Host == "" || config.Redirects.Port == "" {
		utils.Error.Fatal("Config: Redirect host and port must be set")
	}

	server = &http.Server{
		Handler:      router,
		Addr:         config.Redirects.Host + ":" + config.Redirects.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	db.Postgres.CheckTableExists(createTable)

	utils.Info.Printf("Redirects listening on %s", server.Addr)

	return server.ListenAndServe()
}

func Stop() {
	if err := server.Shutdown(context.Background()); err != nil {
		utils.Error.Fatalf("Failed to shutdown server: %v", err)
	}
}

func setRedis(key, data string) {
	err := db.Redis.SetEx(context.Background(), key, data, time.Hour).Err()
	if err != nil {
		utils.Error.Printf("Error caching redirect: %v", err)
	}
}

func getRedis(ctx context.Context, key string) (string, error) {
	data, err := db.Redis.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, db.RedisErrNil) {
		return "", err
	}

	return data, nil
}

func getRedirect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["id"]
	if key == "" {
		http.NotFound(w, r)

		return
	}

	cache, err := getRedis(r.Context(), key)
	if err == nil && cache != "" {
		w.Header().Set("X-Cache-Hit", "HIT")
		http.Redirect(w, r, cache, http.StatusSeeOther)

		return
	} else {
		w.Header().Set("X-Cache-Hit", "MISS")
	}

	redirect, err := db.Postgres.GetRedirectByKey(r.Context(), key)
	if err != nil {
		if errors.Is(err, db.PostgresNoRows) {
			http.NotFound(w, r)

			return
		}

		utils.Error.Printf("Error fetching redirect: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if !strings.HasPrefix(redirect, "https://") {
		redirect = "https://" + redirect
	}

	go setRedis(key, redirect)

	r.Header.Set("X-Cache-Hit", "MISS")
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}
