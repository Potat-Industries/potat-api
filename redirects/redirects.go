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

func getRedirect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["id"]

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

	http.Redirect(w, r, redirect, http.StatusSeeOther)
}
