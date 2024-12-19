package redirects

import (
	"context"
	"net/http"
	"strings"
	"time"

	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"

	"github.com/gorilla/mux"
)

var server *http.Server
var router *mux.Router

func init() {
	router = mux.NewRouter()

	router.Use(middleware.LogRequest)
	router.Use(middleware.GlobalLimiter)
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

	utils.Info.Printf("Redirects listening on %s", server.Addr)

	return server.ListenAndServe()
}

func Stop() {
	if err := server.Shutdown(context.Background()); err != nil {
		utils.Error.Fatalf("Failed to shutdown server: %v", err)
	}
}

func setRedis(ctx context.Context, key, data string) error {
	err := db.Redis.Set(ctx, key, data, 0).Err()
	if err != nil {
		return err
	}

	err = db.Redis.Expire(ctx, key, time.Hour).Err()
	if err != nil {
		return err
	}

	return nil
}

func getRedis(ctx context.Context, key string) (string, error) {
	data, err := db.Redis.Get(ctx, key).Result()
	if err != nil {
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
		http.Redirect(w, r, cache, http.StatusSeeOther)
		return
	}

	redirect, err := db.Postgres.GetRedirectByKey(r.Context(), key)
	if err != nil {
		if err == db.PostgresNoRows {
			http.NotFound(w, r)
			return
		}

		utils.Error.Printf("Error fetching redirect: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(redirect, "http://") && !strings.HasPrefix(redirect, "https://") {
		redirect = "https://" + redirect
	}

	err = setRedis(r.Context(), key, redirect)
	if err != nil {
		utils.Error.Printf("Error caching redirect: %v", err)
	}

	http.Redirect(w, r, redirect, http.StatusSeeOther)
}
