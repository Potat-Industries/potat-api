package haste

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"

	"github.com/gorilla/mux"
)

var (
	keyLength  int
	server     *http.Server
	router     *mux.Router
)

func init() {
	staticPath := loadStaticFilePath()
	staticFiles := loadStaticFiles(staticPath)

	router = mux.NewRouter()

	router.Use(middleware.LogRequest)
	router.Use(middleware.GlobalLimiter)

	router.HandleFunc("/raw/{id}", handleGetRaw).Methods(http.MethodGet)
	router.HandleFunc("/documents", handlePost).Methods(http.MethodPost)
	router.HandleFunc("/documents/{id}", handleGet).Methods(http.MethodGet)
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {	
		if _, exists := staticFiles[r.URL.Path]; !exists { 
			r.URL.Path = "/"
		}
		http.FileServer(http.Dir(staticPath)).ServeHTTP(w, r)
	}).Methods(http.MethodGet)
}

func StartServing(config common.Config) error {
	if config.Haste.Host == "" || config.Haste.Port == "" {
		utils.Error.Fatal("Config: Haste host and port must be set")
	}
	
	if config.Haste.KeyLength != 0 {
		keyLength = config.Haste.KeyLength
	} else {
		keyLength = 6
	}

	server = &http.Server{
		Handler:     router,
		Addr:         config.Haste.Host + ":" + config.Haste.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	utils.Info.Printf("Haste listening on %s",server.Addr)

	return server.ListenAndServe()
}

func Stop() {
	if err :=server.Shutdown(context.Background()); err != nil {
		utils.Error.Fatalf("Failed to shutdown server: %v", err)
	}
}

func getRedis(ctx context.Context, key string) (string, error) {
	data, err := db.Redis.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	return data, nil
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

func loadStaticFilePath() string {
	pwd, err := os.Getwd()
	if err != nil {
		utils.Error.Panic("Failed loading Haste static file path: ", err)
	}

	return filepath.Join(pwd, "./haste/static")
}

func loadStaticFiles(staticPath string) map[string]bool {
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

func handleGet(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["id"]
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		return
	}

	cache, err := getRedis(r.Context(), key)
	if cache != "" && err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache-Hit", "HIT")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"key": key, "data": cache})
		return
	}

	data, err := db.Postgres.GetHaste(r.Context(), key)
	if err != nil || data == "" {
		utils.Warn.Printf("Failed to get document: %v", err)
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	err = setRedis(r.Context(), key, data)
	if err != nil {
		utils.Warn.Printf("Failed to cache document: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache-Hit", "MISS")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"key": key, "data": data})
}

func handleGetRaw(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["id"]
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		return
	}

	cache, err := getRedis(r.Context(), key)
	if cache != "" && err == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Cache-Hit", "HIT")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(cache))
		return
	}

	data, err := db.Postgres.GetHaste(r.Context(), key)
	if err != nil || data == "" {
		utils.Warn.Printf("Failed to get document: %v", err)
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	err = setRedis(r.Context(), key, data)
	if err != nil {
		utils.Warn.Printf("Failed to cache document: %v", err)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Cache-Hit", "MISS")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(data))
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		utils.Warn.Println("Error reading request body: ", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		utils.Warn.Println("Empty body")
		http.Error(w, "Length required", http.StatusLengthRequired)
		return
	}

	key, err := chooseKey(r.Context())
	if err != nil {
		utils.Warn.Println("Failed to generate key: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = db.Postgres.NewHaste(r.Context(), key, body, r.RemoteAddr)
	if err != nil {
		utils.Warn.Println("Failed to save document: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
  json.NewEncoder(w).Encode(map[string]string{"key": key})
}

func chooseKey(ctx context.Context) (string, error) {
	for {
		key, err := utils.RandomString(keyLength)
		if err != nil {
			return "", err
		}

		data, err := db.Postgres.GetHaste(ctx, key)
		if err != nil && err != db.PostgresNoRows {
			return "", err
		}

		if data == "" {
			return key, nil
		}
	}
}