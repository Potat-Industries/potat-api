// Package get contains routes for http.MethodGet requests.
package get

import (
	"net/http"
	"time"

	"github.com/Potat-Industries/potat-api/api"
	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/gorilla/mux"
)

// CommandSettingsResponse is the response type for GET /channel/command-settings.
type CommandSettingsResponse = common.GenericResponse[common.CommandSettings]

func init() {
	api.SetRoute(api.Route{
		Path:    "/channel/command-settings",
		Method:  http.MethodGet,
		Handler: getCommandSettingsHandler,
		UseAuth: true,
	})
}

// resolveChannelID returns the channel ID from ?id= or defaults to the authenticated user's Twitch ID.
func resolveChannelID(request *http.Request, user *common.User) string {
	if id := request.URL.Query().Get("id"); id != "" {
		return id
	}

	// Check path variable as fallback (e.g. /channel/:id/...)
	if id := mux.Vars(request)["id"]; id != "" {
		return id
	}

	return middleware.GetTwitchPlatformID(user)
}

func getCommandSettingsHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := request.Context().Value(middleware.AuthedUser).(*common.User)
	if !ok || user == nil {
		api.GenericResponse(writer, http.StatusUnauthorized, CommandSettingsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK {
		logger.Error.Println("Postgres client not found in context")
		api.GenericResponse(writer, http.StatusInternalServerError, CommandSettingsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)

		return
	}

	channelID := resolveChannelID(request, user)
	if channelID == "" {
		api.GenericResponse(writer, http.StatusBadRequest, CommandSettingsResponse{
			Errors: &[]common.ErrorMessage{{Message: "channel id is required"}},
		}, start)

		return
	}

	if !middleware.IsChannelAuthorized(request, user, channelID, postgres) {
		api.GenericResponse(writer, http.StatusForbidden, CommandSettingsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Forbidden"}},
		}, start)

		return
	}

	settings, err := postgres.GetCommandSettings(request.Context(), channelID)
	if err != nil {
		logger.Error.Printf("Error fetching command settings: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, CommandSettingsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Failed to fetch command settings"}},
		}, start)

		return
	}

	if settings == nil {
		settings = []common.CommandSettings{}
	}

	api.GenericResponse(writer, http.StatusOK, CommandSettingsResponse{
		Data: &settings,
	}, start)
}
