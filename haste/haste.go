package haste

import (
	"io"
	"os"
	"time"
	"context"
	"net/http"
	"path/filepath"
	"encoding/json"

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
	router = mux.NewRouter()

	router.Use(middleware.LogRequest)
	router.Use(middleware.GlobalLimiter)

	router.HandleFunc("/raw/{id}", handleGetRaw).Methods(http.MethodGet)
	router.HandleFunc("/documents", handlePost).Methods(http.MethodPost)
	router.HandleFunc("/documents/{id}", handleGet).Methods(http.MethodGet)

	pwd, err := os.Getwd()
	if err != nil {
		utils.Error.Panic("Failed loading Haste static file path: ", err)
	}

	staticPath := filepath.Join(pwd, "./haste/static")

  

	router.HandleFunc("/{id}", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/"
		
		http.FileServer(http.Dir(staticPath)).ServeHTTP(w, r)
	}).Methods(http.MethodGet)

	router.PathPrefix("/").Handler(http.FileServer(http.Dir(staticPath)))
}

func StartServing(config common.Config) error {
	if config.Haste.Host == "" || config.Haste.Port == "" {
		utils.Error.Fatal("Config: Redirect host and port must be set")
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

func handleGet(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["id"]
	data, err := db.Postgres.GetHaste(r.Context(), key)
	if err != nil || data == "" {
		utils.Warn.Printf("Failed to get document: %v", err)
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"key": key, "data": data})
}

func handleGetRaw(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["id"]
	data, err := db.Postgres.GetHaste(r.Context(), key)

	if err != nil || data == "" {
		utils.Warn.Printf("Failed to get document: %v", err)
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
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