package api

import (
	"context"
	"net/http"
	"potat-api/api/middleware"
	"potat-api/api/utils"
	"time"

	"github.com/gorilla/mux"
)

var (
	Server *http.Server
)

type Route struct {
	Path string
	Method string
	Handler http.HandlerFunc
}

var Routes = []Route{}

func init() {
	router := mux.NewRouter()

	router.Use(middleware.LogRequest)
	router.Use(middleware.GlobalLimiter)

	Server = &http.Server{
		Handler: router,
		Addr:    "localhost:3111",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func Stop() {
	utils.Debug.Fatal(Server.Shutdown(context.Background()))
}

func StartServing() {
	utils.Debug.Fatal(Server.ListenAndServe())
}

func SetRoute(route Route) {
	router := Server.Handler.(*mux.Router)
	router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
}