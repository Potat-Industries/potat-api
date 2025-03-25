// Package haste provides a simple pastebin-like service for storing and retrieving files.
package haste

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"
)

const createTable = `
	CREATE TABLE IF NOT EXISTS haste (
		key char(32) UNIQUE NOT NULL,
		content BYTEA NOT NULL,
		access_count INT DEFAULT 1 NOT NULL,
		source TEXT default 'potatbotat',
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
`

type hastebin struct {
	server    *http.Server
	router    *mux.Router
	postgres  *db.PostgresClient
	redis     *db.RedisClient
	keyLength int
}

// StartServing will start the Haste server on the configured port.
func StartServing(config common.Config, postgres *db.PostgresClient, redis *db.RedisClient) error {
	if config.Haste.Host == "" || config.Haste.Port == "" {
		utils.Error.Fatal("Config: Haste host and port must be set")
	}

	haste := hastebin{
		keyLength: 6,
		postgres:  postgres,
		redis:     redis,
	}

	router := mux.NewRouter()

	limiter := middleware.NewRateLimiter(100, 1*time.Minute, redis)
	router.Use(middleware.LogRequest)
	router.Use(limiter)

	staticPath := haste.loadStaticFilePath()
	staticFiles := haste.loadStaticFiles(staticPath)

	router.HandleFunc("/raw/{id}", haste.handleGetRaw).Methods(http.MethodGet)
	router.HandleFunc("/documents", haste.handlePost).Methods(http.MethodPost)
	router.HandleFunc("/documents/{id}", haste.handleGet).Methods(http.MethodGet)
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, exists := staticFiles[r.URL.Path]; !exists {
			r.URL.Path = "/"
		}
		http.FileServer(http.Dir(staticPath)).ServeHTTP(w, r)
	}).Methods(http.MethodGet)

	haste.server = &http.Server{
		Handler:      router,
		Addr:         config.Haste.Host + ":" + config.Haste.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	haste.router = router

	if config.Haste.KeyLength != 0 {
		haste.keyLength = config.Haste.KeyLength
	}

	haste.postgres.CheckTableExists(createTable)

	utils.Info.Printf("Haste listening on %s", haste.server.Addr)

	return haste.server.ListenAndServe()
}

func (h *hastebin) getRedis(ctx context.Context, key string) (string, error) {
	data, err := h.redis.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	return data, nil
}

func (h *hastebin) setRedis(ctx context.Context, key, data string) {
	err := h.redis.SetEx(ctx, key, data, time.Hour).Err()
	if err != nil {
		utils.Warn.Printf("Failed to cache document: %v", err)

		return
	}
}

func (h *hastebin) loadStaticFilePath() string {
	pwd, err := os.Getwd()
	if err != nil {
		utils.Error.Panic("Failed loading Haste static file path: ", err)
	}

	return filepath.Join(pwd, "./haste/static")
}

func (h *hastebin) loadStaticFiles(staticPath string) map[string]bool {
	files := make(map[string]bool)

	err := filepath.Walk(staticPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relativePath := strings.TrimPrefix(path, staticPath)
			files[relativePath] = true
		}

		return nil
	})
	if err != nil {
		utils.Error.Fatalf("Failed to load static files: %v", err)
	}

	return files
}

func (h *hastebin) handleGet(writer http.ResponseWriter, request *http.Request) {
	key := mux.Vars(request)["id"]
	if key == "" {
		http.Error(writer, "Key not provided", http.StatusBadRequest)

		return
	}

	cache, err := h.getRedis(request.Context(), key)
	if cache != "" && err == nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Cache-Hit", "HIT")
		writer.WriteHeader(http.StatusOK)
		err = json.NewEncoder(writer).Encode(map[string]string{"key": key, "data": cache})
		if err != nil {
			utils.Warn.Println("Failed to write document: ", err)
		}

		return
	}

	data, err := h.postgres.GetHaste(request.Context(), key)
	if err != nil || data == "" {
		utils.Warn.Printf("Failed to get document: %v", err)
		http.Error(writer, "Document not found", http.StatusNotFound)

		return
	}

	go h.setRedis(request.Context(), key, data)

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Cache-Hit", "MISS")
	writer.WriteHeader(http.StatusOK)
	err = json.NewEncoder(writer).Encode(map[string]string{"key": key, "data": data})
	if err != nil {
		utils.Warn.Println("Failed to write document: ", err)
	}
}

func (h *hastebin) handleGetRaw(writer http.ResponseWriter, request *http.Request) {
	key := mux.Vars(request)["id"]
	if key == "" {
		http.Error(writer, "Key not provided", http.StatusBadRequest)

		return
	}

	if strings.Contains(key, ".") {
		key = strings.Split(key, ".")[0]
	}

	cache, err := h.getRedis(request.Context(), key)
	if cache != "" && err == nil {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.Header().Set("X-Cache-Hit", "HIT")
		writer.WriteHeader(http.StatusOK)
		_, err = writer.Write([]byte(cache))
		if err != nil {
			utils.Warn.Println("Failed to write document: ", err)
		}

		return
	}

	data, err := h.postgres.GetHaste(request.Context(), key)
	if err != nil || data == "" {
		utils.Warn.Printf("Failed to get document: %v", err)
		http.Error(writer, "Document not found", http.StatusNotFound)

		return
	}

	go h.setRedis(request.Context(), key, data)

	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.Header().Set("X-Cache-Hit", "MISS")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write([]byte(data))
	if err != nil {
		utils.Warn.Println("Failed to write document: ", err)
	}
}

func (h *hastebin) handlePost(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		http.Error(writer, "Error parsing form", http.StatusBadRequest)

		return
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		utils.Warn.Println("Error reading request body: ", err)
		http.Error(writer, "Error reading request body", http.StatusInternalServerError)

		return
	}
	defer func() {
		if err = request.Body.Close(); err != nil {
			utils.Warn.Println("Error closing hastebin request body: ", err)
		}
	}()

	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil {
		utils.Warn.Println("Error parsing media type: ", err)
		http.Error(writer, "Invalid content type", http.StatusUnsupportedMediaType)

		return
	}

	allowedTypes := map[string]bool{
		"text/plain":       true,
		"text/markdown":    true,
		"text/x-markdown":  true,
		"application/json": true,
	}

	if !allowedTypes[mediaType] {
		utils.Warn.Println("Invalid media type: ", mediaType)
		http.Error(writer, "Invalid media type", http.StatusUnsupportedMediaType)

		return
	}

	if len(body) == 0 {
		utils.Warn.Println("Empty body")
		http.Error(writer, "Length required", http.StatusLengthRequired)

		return
	}

	key, err := h.chooseKey(request.Context())
	if err != nil {
		utils.Warn.Println("Failed to generate key: ", err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)

		return
	}

	err = h.postgres.NewHaste(request.Context(), key, body, request.RemoteAddr)
	if err != nil {
		utils.Warn.Println("Failed to save document: ", err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)

		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	err = json.NewEncoder(writer).Encode(map[string]string{"key": key})
	if err != nil {
		utils.Warn.Println("Failed to write response: ", err)
	}
}

func (h *hastebin) chooseKey(ctx context.Context) (string, error) {
	for {
		key, err := utils.RandomString(h.keyLength)
		if err != nil {
			return "", err
		}

		data, err := h.postgres.GetHaste(ctx, key)
		if err != nil && !errors.Is(err, db.PostgresNoRows) {
			return "", err
		}

		if data == "" {
			return key, nil
		}
	}
}
