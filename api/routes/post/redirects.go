package post

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"potat-api/api"
	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/logger"
	"potat-api/common/utils"
)

func init() {
	api.SetRoute(api.Route{
		Path:    "/redirect",
		Method:  http.MethodPost,
		Handler: createRedirect,
		UseAuth: false,
	})
}

func createRedirect(w http.ResponseWriter, r *http.Request) {
	var input common.Redirect
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		logger.Error.Printf("Invalid request body: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		input.URL = "https://" + input.URL
	}

	postgres, ok := r.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")

		return
	}

	key, err := postgres.GetKeyByRedirect(r.Context(), input.URL)
	if err == nil && key != "" {
		response := fmt.Sprintf("https://%s/%s", r.Host, key)
		_, err := w.Write([]byte(response))
		if err != nil {
			logger.Error.Printf("Failed to write response: %v", err)
		}

		return
	}

	key, err = generateUniqueKey(r.Context())
	if err != nil {
		logger.Error.Printf("Error generating key: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if err := postgres.NewRedirect(r.Context(), key, input.URL); err != nil {
		logger.Error.Printf("Error inserting redirect: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	response := fmt.Sprintf("https://%s/%s", r.Host, key)
	_, err = w.Write([]byte(response))
	if err != nil {
		logger.Error.Printf("Failed to write response: %v", err)
	}
}

func generateUniqueKey(ctx context.Context) (string, error) {
	postgres, ok := ctx.Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")

		return "", middleware.ErrMissingContext
	}

	for {
		key, err := utils.RandomString(6)
		if err != nil {
			return "", err
		}
		if postgres.RedirectExists(ctx, key) {
			return key, nil
		}
	}
}
