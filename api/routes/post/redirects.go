// Package post contains routes for http.MethodPost requests.
package post

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Potat-Industries/potat-api/api"
	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/Potat-Industries/potat-api/common/utils"
)

func init() {
	api.SetRoute(api.Route{
		Path:    "/redirect",
		Method:  http.MethodPost,
		Handler: createRedirect,
		UseAuth: false,
	})
}

func createRedirect(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop
	var input common.Redirect
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		logger.Error.Printf("Invalid request body: %v", err)
		http.Error(writer, "Bad Request", http.StatusBadRequest)

		return
	}

	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		input.URL = "https://" + input.URL
	}

	postgres, ok := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")

		return
	}

	key, err := postgres.GetKeyByRedirect(request.Context(), input.URL)
	if err == nil && key != "" {
		response := fmt.Sprintf("https://%s/%s", request.Host, key)
		_, err = writer.Write([]byte(response))
		if err != nil {
			logger.Error.Printf("Failed to write response: %v", err)
		}

		return
	}

	key, err = generateUniqueKey(request.Context())
	if err != nil {
		logger.Error.Printf("Error generating key: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if err = postgres.NewRedirect(request.Context(), key, input.URL); err != nil {
		logger.Error.Printf("Error inserting redirect: %v", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	response := fmt.Sprintf("https://%s/%s", request.Host, key)
	if _, err = writer.Write([]byte(response)); err != nil {
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
