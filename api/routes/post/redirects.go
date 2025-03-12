package post

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"potat-api/api"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"
)

func init() {
	api.SetRoute(api.Route{
		Path:    "/redirect",
		Method:  http.MethodPost,
		Handler: createRedirect,
	}, false)
}

func createRedirect(w http.ResponseWriter, r *http.Request) {
	var input common.Redirect
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error.Printf("Invalid request body: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		input.URL = "https://" + input.URL
	}

	key, err := db.Postgres.GetKeyByRedirect(r.Context(), input.URL)
	if err == nil && key != "" {
		response := fmt.Sprintf("https://%s/%s", r.Host, key)
		_, err := w.Write([]byte(response))
		if err != nil {
			utils.Error.Printf("Failed to write response: %v", err)
		}

		return
	}

	key, err = generateUniqueKey(r.Context())
	if err != nil {
		utils.Error.Printf("Error generating key: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if err := db.Postgres.NewRedirect(r.Context(), key, input.URL); err != nil {
		utils.Error.Printf("Error inserting redirect: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	response := fmt.Sprintf("https://%s/%s", r.Host, key)
	_, err = w.Write([]byte(response))
	if err != nil {
		utils.Error.Printf("Failed to write response: %v", err)
	}
}

func generateUniqueKey(ctx context.Context) (string, error) {
	for {
		key, err := utils.RandomString(6)
		if err != nil {
			return "", err
		}
		if db.Postgres.RedirectExists(ctx, key) {
			return key, nil
		}
	}
}
