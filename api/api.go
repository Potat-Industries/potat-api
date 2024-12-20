package api

import (
	"time"
	"context"
	"net/http"

	"potat-api/common"
	"potat-api/common/utils"
	"potat-api/api/middleware"

	"github.com/gorilla/mux"
)
type Route struct {
	Path string
	Method string
	Handler http.HandlerFunc
}

var server *http.Server
var router *mux.Router

func init() {
	router = mux.NewRouter()

	router.Use(middleware.LogRequest)
	router.Use(middleware.GlobalLimiter)
}

func Stop() {
	utils.Debug.Fatal(server.Shutdown(context.Background()))
}

func StartServing(config common.Config) error {
	if config.API.Host == "" || config.API.Port == "" {
		utils.Error.Fatal("Config: API host and port must be set")
	}
	
	server = &http.Server{
		Handler: router,
		Addr:    config.API.Host + ":" + config.API.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	utils.Info.Printf("API listening on %s", server.Addr)

	return server.ListenAndServe()
}

func SetRoute(route Route) {
	router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
}