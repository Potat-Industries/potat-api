// Package api provides a direct and proxied api for PotatBotat
package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/Potat-Industries/potat-api/common/utils"
	"github.com/gorilla/mux"
)

type Route struct {
	Handler http.HandlerFunc
	Path    string
	Method  string
	UseAuth bool
}

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

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Add("Vary", "Origin")
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func csrfMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	mutating := map[string]struct{}{
		http.MethodPost:   {},
		http.MethodPut:    {},
		http.MethodPatch:  {},
		http.MethodDelete: {},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, isMutating := mutating[r.Method]; isMutating {
				origin := r.Header.Get("Origin")
				if origin != "" {
					if _, ok := allowed[origin]; !ok {
						http.Error(w, "Forbidden", http.StatusForbidden)

						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func StartServing(
	config common.Config,
	postgres *db.PostgresClient,
	redis *db.RedisClient,
	clickhouse *db.ClickhouseClient,
	nats *utils.NatsClient,
	metrics *utils.Metrics,
) error {
	if config.API.Host == "" || config.API.Port == "" {
		logger.Error.Fatal("Config: API host and port must be set")
	}

	api := Server{
		router: mux.NewRouter(),
	}

	api.router.Use(corsMiddleware(config.API.CORSOrigins))
	api.router.Use(middleware.LogRequest(metrics))
	api.router.Use(middleware.InjectDatabases(postgres, redis, clickhouse, nats))
	api.router.Use(middleware.NewRateLimiter(100, 1*time.Minute, redis))

	authenticator := middleware.NewAuthenticator(config.Twitch.ClientSecret, GenericResponse)
	api.router.Use(authenticator.InjectAuthenticator())

	api.authedRouter = api.router.PathPrefix("/").Subrouter()
	api.authedRouter.Use(csrfMiddleware(config.API.CORSOrigins))
	api.authedRouter.Use(authenticator.SetDynamicAuthMiddleware())

	api.server = &http.Server{
		Handler:      api.router,
		Addr:         config.API.Host + ":" + config.API.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info.Printf("API listening on %s", api.server.Addr)

	for _, route := range registry.routes {
		logger.Info.Printf("Registering route: %s %s", route.Method, route.Path)
		api.registerRoute(route)
	}

	api.router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		GenericResponse(w, http.StatusNotFound, map[string]string{"error": "Not Found"}, time.Now())
	})

	return api.server.ListenAndServe()
}

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

func GenericResponse(
	writer http.ResponseWriter,
	code int,
	response any,
	start time.Time,
) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Request-Duration", time.Since(start).String())
	writer.WriteHeader(code)

	err := json.NewEncoder(writer).Encode(response)
	if err != nil {
		logger.Error.Printf("Error encoding response: %v", err)
	}
}
