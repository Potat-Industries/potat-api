// Package uploader provides a simple private http server for uploading and retrieving files.
package uploader

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/Potat-Industries/potat-api/common/utils"
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

type uploader struct {
	server        *http.Server
	router        *mux.Router
	hasher        func(string) string
	postgres      *db.PostgresClient
	redis         *db.RedisClient
	cacheDuration time.Duration
	keyLength     int
}

type upload struct {
	Key        string `json:"key"`
	URL        string `json:"url"`
	DeleteHash string `json:"delete_hash"`
}

func getHashGenerator(secret string) func(key string) string {
	return func(key string) string {
		hash := crypto.SHA256.New()
		hash.Write([]byte(key + secret))

		return fmt.Sprintf("%x", hash.Sum(nil))
	}
}

// StartServing will start the uploader server on the configured port.
func StartServing(
	ctx context.Context,
	config common.Config,
	postgres *db.PostgresClient,
	redis *db.RedisClient,
	metrics *utils.Metrics,
) error {
	if config.Uploader.Host == "" || config.Uploader.Port == "" {
		logger.Error.Fatal("Config: Uploader host and port must be set")
	}

	uploader := &uploader{
		keyLength:     6,
		cacheDuration: 30 * time.Minute,
		hasher:        getHashGenerator(config.Uploader.AuthKey),
		postgres:      postgres,
		redis:         redis,
	}

	router := mux.NewRouter()

	router.Use(middleware.LogRequest(metrics))
	router.Use(middleware.NewRateLimiter(200, 1*time.Minute, redis))
	router.HandleFunc("/{key}", uploader.handleGet).Methods(http.MethodGet)

	deleteRouter := router.PathPrefix("/delete").Subrouter()
	deleteRouter.Use(middleware.NewRateLimiter(15, 1*time.Minute, redis))
	deleteRouter.HandleFunc("/{key}/{hash}", uploader.handleDelete).Methods(http.MethodGet)

	authedRoute := router.PathPrefix("/").Subrouter()
	authedRoute.HandleFunc("/upload", uploader.handleUpload).Methods(http.MethodPost)

	authenicator := middleware.NewAuthenticator(config.Uploader.AuthKey, nil)
	authedRoute.Use(authenicator.SetStaticAuthMiddleware())
	authedRoute.Use(middleware.NewRateLimiter(25, 1*time.Minute, redis))

	uploader.server = &http.Server{
		Handler:      router,
		Addr:         config.Uploader.Host + ":" + config.Uploader.Port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	uploader.router = router

	if config.Haste.KeyLength != 0 {
		uploader.keyLength = config.Haste.KeyLength
	}

	uploader.postgres.CheckTableExists(ctx, createTable)

	logger.Info.Printf("Uploader listening on %s", uploader.server.Addr)

	return uploader.server.ListenAndServe()
}

func (u *uploader) setRedis(ctx context.Context, key string, data []byte) {
	err := u.redis.SetEx(ctx, key, data, u.cacheDuration).Err()
	if err != nil {
		logger.Warn.Printf("Failed to cache document: %v", err)
	}
}

func (u *uploader) handleUpload(writer http.ResponseWriter, request *http.Request) {
	err := request.ParseMultipartForm(maxFileSize)
	if err != nil {
		logger.Error.Printf("Error parsing form: %v", err)
		http.Error(writer, "Failed to parse form", http.StatusBadRequest)

		return
	}

	file, header, err := request.FormFile("file")
	if err != nil {
		logger.Error.Printf("Error retrieving file: %v", err)
		http.Error(writer, "File is required", http.StatusBadRequest)

		return
	}
	defer func() {
		if err = file.Close(); err != nil {
			logger.Error.Printf("Error closing file: %v", err)
		}
	}()

	fileName := header.Filename
	fileData, err := io.ReadAll(file)
	if err != nil {
		logger.Error.Printf("Error reading file: %v", err)
		http.Error(writer, "Failed to read file", http.StatusInternalServerError)

		return
	}

	mimeType := http.DetectContentType(fileData)

	key, err := utils.RandomString(u.keyLength)
	if err != nil {
		logger.Error.Printf("Error generating key: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	ok, createdAt := u.postgres.NewUpload(
		request.Context(),
		key,
		fileData,
		mimeType,
		fileName,
	)
	if !ok {
		logger.Error.Printf("Error inserting upload: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	parentCtx := context.WithoutCancel(request.Context())
	go u.setRedis(parentCtx, key, fileData)

	response := fmt.Sprintf("https://%s/%s", request.Host, key)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)

	err = json.NewEncoder(writer).Encode(upload{
		Key:        key,
		URL:        response,
		DeleteHash: u.hasher(key + createdAt.String()),
	})
	if err != nil {
		logger.Error.Printf("Error encoding response: %v", err)
	}
}

func (u *uploader) handleDelete(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	key := vars["key"]
	hash := vars["hash"]

	u.redis.Del(request.Context(), key)

	createdAt, err := u.postgres.GetUploadCreatedAt(request.Context(), key)
	if err != nil {
		http.Error(writer, "Not Found", http.StatusNotFound)

		return
	}

	if hash != u.hasher(key+createdAt.String()) {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)

		return
	}

	ok := u.postgres.DeleteFileByKey(request.Context(), key)
	if !ok {
		http.Error(writer, "Not Found", http.StatusNotFound)

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (u *uploader) handleGet(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	key := vars["key"]

	cache, err := u.redis.Get(request.Context(), key).Bytes()
	if cache != nil && err == nil {
		contentType := http.DetectContentType(cache)
		writer.Header().Set("Content-Type", contentType)
		writer.Header().Set("X-Cache-Hit", "HIT")
		writer.WriteHeader(http.StatusOK)
		_, err = writer.Write(cache)
		if err != nil {
			logger.Warn.Printf("Failed to write document: %v", err)
		}

		return
	}

	data, mimeType, name, _, err := u.postgres.GetFileByKey(request.Context(), key)
	if errors.Is(err, db.ErrPostgresNoRows) {
		http.Error(writer, "Not Found", http.StatusNotFound)

		return
	}

	if err != nil {
		logger.Warn.Printf("Failed to get document: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	parentCtx := context.WithoutCancel(request.Context())
	go u.setRedis(parentCtx, key, data)

	if name != nil {
		writer.Header().Set("Content-Disposition", "inline; filename=\""+*name+"\"")
	}
	writer.Header().Set("Content-Type", mimeType)
	_, err = writer.Write(data)
	if err != nil {
		logger.Error.Printf("Error writing file: %v", err)
		http.Error(writer, "Failed to write file", http.StatusInternalServerError)
	}
}
