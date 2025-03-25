// Package api provides a direct and proxied api for PotatBotat
package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"
)

// Route represents a single API route with its handler, path, method, and authentication requirement.
type Route struct {
	Handler http.HandlerFunc
	Path    string
	Method  string
	UseAuth bool
}

// Server represents the API server, including the main router and an authenticated sub-router.
type Server struct {
	server       *http.Server
	router       *mux.Router
	authedRouter *mux.Router
}

type register struct {
	routes []Route
	mu     sync.Mutex
}

var registry = &register{} //nolint:gochecknoglobals // Used to conveniently register API routes.

// StartServing initializes and starts the API server with the configured routes and middleware.
func StartServing(
	config common.Config,
	postgres *db.PostgresClient,
	redis *db.RedisClient,
	clickhouse *db.ClickhouseClient,
) error {
	if config.API.Host == "" || config.API.Port == "" {
		utils.Error.Fatal("Config: API host and port must be set")
	}

	api := Server{
		router: mux.NewRouter(),
	}
	
	api.router.Use(middleware.LogRequest)
	api.router.Use(middleware.InjectDatabases(postgres, redis, clickhouse))
	api.router.Use(middleware.NewRateLimiter(100, 1*time.Minute, redis))
	
	authenticator := middleware.NewAuthenticator(config.Twitch.ClientSecret, GenericResponse)
	api.authedRouter = api.router.PathPrefix("/").Subrouter()
	api.authedRouter.Use(authenticator.SetDynamicAuthMiddleware())

	api.server = &http.Server{
		Handler:      api.router,
		Addr:         config.API.Host + ":" + config.API.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	utils.Info.Printf("API listening on %s", api.server.Addr)

	for _, route := range registry.routes {
		utils.Info.Printf("Registering route: %s %s", route.Method, route.Path)
		api.registerRoute(route)
	}

	// Catch-all for unmatched routes
	api.router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		GenericResponse(w, http.StatusNotFound, map[string]string{"error": "Not Found"}, time.Now())
	})

	return api.server.ListenAndServe()
}

// SetRoute adds a new route to the registry.
func SetRoute(route Route) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.routes = append(registry.routes, route)
}

func (a *Server) registerRoute(route Route) {
	if route.UseAuth {
		a.authedRouter.HandleFunc(route.Path, route.Handler).Methods(route.Method)

		return
	}
	a.router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
}

// GenericResponse is a utility function to send a JSON response with a specified status code and duration.
func GenericResponse(
	writer http.ResponseWriter,
	code int,
	response interface{},
	start time.Time,
) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Request-Duration", time.Since(start).String())
	writer.WriteHeader(code)

	err := json.NewEncoder(writer).Encode(response)
	if err != nil {
		utils.Error.Printf("Error encoding response: %v", err)
	}
}
