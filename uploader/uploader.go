package uploader

import (
	"io"
	"fmt"
	"time"
	"crypto"
	"context"
	"net/http"
	"encoding/json"

	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"
	"potat-api/api/middleware"

	"github.com/gorilla/mux"
)

const maxFileSize = 20971520 // 20MB

const createTable = `
	CREATE TABLE IF NOT EXISTS file_store (
		key VARCHAR(50) PRIMARY KEY,
		file BYTEA NOT NULL,
		file_name VARCHAR(50),
		mime_type VARCHAR(50) NOT NULL,
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT NOW() NOT NULL
	);
`

var server *http.Server
var router *mux.Router
var hasher func(string) string
var cacheDuration time.Duration
var keyLength = 6

type Upload struct {
	Key string 					`json:"key"`
	URL string					`json:"url"`
	DeleteHash string 	`json:"delete_hash"`
}

func init() {
	router = mux.NewRouter()

	router.Use(middleware.LogRequest)
	router.Use(middleware.NewRateLimiter(200, 1 * time.Minute))
	router.HandleFunc("/{key}", handleGet).Methods(http.MethodGet)

	deleteRouter := router.PathPrefix("/delete").Subrouter()
	deleteRouter.Use(middleware.NewRateLimiter(15, 1 * time.Minute))
	deleteRouter.HandleFunc("/{key}/{hash}", handleDelete).Methods(http.MethodGet)
}

func StartServing(config common.Config) error {
	if config.Uploader.Host == "" || config.Uploader.Port == "" {
		utils.Error.Fatal("Config: Uploader host and port must be set")
	}

	keyLength = config.Haste.KeyLength
	cacheDuration = 30 * time.Minute // TODO: Make this configurable?
	hasher = getHashGenerator(config.Uploader.AuthKey)

	authedRoute := router.PathPrefix("/").Subrouter()
	authedRoute.HandleFunc("/upload", handleUpload).Methods(http.MethodPost)
	authedRoute.Use(middleware.SetAuthMiddleware(config.Uploader.AuthKey))
	authedRoute.Use(middleware.NewRateLimiter(25, 1 * time.Minute))

	server = &http.Server{
		Handler:      router,
		Addr:         config.Uploader.Host + ":" + config.Uploader.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	db.Postgres.CheckTableExists(createTable)

	utils.Info.Printf("Uploader listening on %s", server.Addr)

	return server.ListenAndServe()
}

func Stop() {
	if err := server.Shutdown(context.Background()); err != nil {
		utils.Error.Fatalf("Failed to shutdown server: %v", err)
	}
}

func setRedis(key string, data []byte) {
	err := db.Redis.SetEx(context.Background(), key, data, cacheDuration).Err()
	if err != nil {
		utils.Warn.Printf("Failed to cache document: %v", err)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(maxFileSize)
	if err != nil {
		utils.Error.Printf("Error parsing form: %v", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.Error.Printf("Error retrieving file: %v", err)
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileName := header.Filename
	fileData, err := io.ReadAll(file)
	if err != nil {
		utils.Error.Printf("Error reading file: %v", err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	mimeType := http.DetectContentType(fileData)

	key, err := utils.RandomString(keyLength)
	if err != nil {
		utils.Error.Printf("Error generating key: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	ok, createdAt := db.Postgres.NewUpload(
		r.Context(),
		key,
		fileData,
		mimeType,
		fileName,
	)
	if !ok {
		utils.Error.Printf("Error inserting upload: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	go setRedis(key, fileData)

	response := fmt.Sprintf("https://%s/%s", r.Host, key)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(Upload{
		Key: key,
		URL: response,
		DeleteHash: hasher(key + createdAt.String()),
	})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]
	hash := vars["hash"]

	db.Redis.Del(r.Context(), key)

	createdAt, err := db.Postgres.GetUploadCreatedAt(r.Context(), key)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if hash != hasher(key + createdAt.String()) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ok := db.Postgres.DeleteFileByKey(r.Context(), key)
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	cache, err := db.Redis.Get(r.Context(), key).Bytes()
	if cache != nil && err == nil {
		contentType := http.DetectContentType(cache)
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Cache-Hit", "HIT")
		w.WriteHeader(http.StatusOK)
		w.Write(cache)
		return
	}

	data, mimeType, name, _, err := db.Postgres.GetFileByKey(r.Context(), key)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	go setRedis(key, data)

	w.Header().Set("Content-Disposition", "inline; filename=\""+name+"\"")
	w.Header().Set("Content-Type", mimeType)
	w.Write(data)
}

func getHashGenerator(secret string) func(key string) string {
	return func(key string) string {
		hash := crypto.SHA256.New()
		hash.Write([]byte(key + secret))

		return fmt.Sprintf("%x", hash.Sum(nil))
	}
}
