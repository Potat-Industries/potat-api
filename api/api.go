package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/utils"

	"github.com/gorilla/mux"
)
type Route struct {
	Path string
	Method string
	Handler http.HandlerFunc
}

var server *http.Server
var router *mux.Router
var authedRouter *mux.Router

func init() {
	router = mux.NewRouter()

	limiter := middleware.NewRateLimiter(100, 1 * time.Minute)
	router.Use(middleware.LogRequest)
	router.Use(limiter)

	authedRouter = router.PathPrefix("/").Subrouter()
	authedRouter.Use(middleware.SetDynamicAuthMiddleware())
}

func Stop() {
	utils.Debug.Fatal(server.Shutdown(context.Background()))
}

func StartServing(config common.Config) error {
	if config.API.Host == "" || config.API.Port == "" {
		utils.Error.Fatal("Config: API host and port must be set")
	}

	middleware.SetJWTSecret(config.Twitch.ClientSecret)

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

func SetRoute(route Route, auth bool) {
	if auth {
		authedRouter.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		return
	}
	router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
}

func GenericResponse(
	r http.ResponseWriter,
	code int,
	response interface{},
	start time.Time,
) {
	r.WriteHeader(code)
	r.Header().Set("Content-Type", "application/json")
	r.Header().Set("X-Potat-Request-Duration", time.Since(start).String())

	err := json.NewEncoder(r).Encode(response)
	if err != nil {
		utils.Error.Printf("Error encoding response: %v", err)
	}
}
