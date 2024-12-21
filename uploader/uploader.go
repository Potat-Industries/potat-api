package uploader

import (
	"context"
	"net/http"
	"time"

	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/utils"

	"github.com/gorilla/mux"
)

var server *http.Server
var router *mux.Router

func init() {
	router = mux.NewRouter()

	limiter := middleware.NewRateLimiter(100, 1 * time.Minute)
	router.Use(middleware.LogRequest)
	router.Use(limiter)
}

func StartServing(config common.Config) error {
	if config.Uploader.Host == "" || config.Uploader.Port == "" {
		utils.Error.Fatal("Config: Uploader host and port must be set")
	}

	server = &http.Server{
		Handler:      router,
		Addr:         config.Uploader.Host + ":" + config.Uploader.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	utils.Info.Printf("Uploader listening on %s", server.Addr)

	return server.ListenAndServe()
}

func Stop() {
	if err := server.Shutdown(context.Background()); err != nil {
		utils.Error.Fatalf("Failed to shutdown server: %v", err)
	}
}