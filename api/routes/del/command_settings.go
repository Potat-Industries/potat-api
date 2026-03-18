// Package del contains routes for http.MethodDelete requests.
package del

import (
	"net/http"
	"slices"
	"time"

	"github.com/Potat-Industries/potat-api/api"
	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
)

func init() {
	api.SetRoute(api.Route{
		Path:    "/channel/command-settings",
		Method:  http.MethodDelete,
		Handler: deleteCommandSettings,
		UseAuth: true,
	})
}

func deleteCommandSettings(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop
	start := time.Now()

	user, ok := request.Context().Value(middleware.AuthedUser).(*common.User)
	if !ok || user == nil {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK {
		logger.Error.Println("Postgres client not found in context")
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)

		return
	}

	channelID := request.URL.Query().Get("id")
	if channelID == "" {
		for _, conn := range user.Connections {
			if conn.Platform == common.TWITCH {
				channelID = conn.UserID

				break
			}
		}
	}

	command := request.URL.Query().Get("command")

	if channelID == "" {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "channel id is required"}},
		}, start)

		return
	}

	if !isDelAuthorized(request, user, channelID, postgres) {
		api.GenericResponse(writer, http.StatusForbidden, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Forbidden"}},
		}, start)

		return
	}

	// command is required — this endpoint resets a single command override back to defaults.
	if command == "" {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Missing required field: command"}},
		}, start)

		return
	}

	if err := postgres.ResetCommandSettings(request.Context(), channelID, command); err != nil {
		logger.Error.Printf("Error resetting command settings: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Failed to reset command settings"}},
		}, start)

		return
	}

	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[any]{
		Data: &[]any{},
	}, start)
}

func isDelAuthorized(
	request *http.Request,
	user *common.User,
	channelID string,
	postgres *db.PostgresClient,
) bool {
	if user.Level >= int(common.ADMIN) {
		return true
	}

	var twitchID string
	for _, conn := range user.Connections {
		if conn.Platform == common.TWITCH {
			twitchID = conn.UserID

			break
		}
	}

	if twitchID == channelID {
		return true
	}

	ambassadors, err := postgres.GetChannelAmbassadors(request.Context(), channelID, common.TWITCH)
	if err != nil {
		return false
	}

	return slices.Contains(ambassadors, twitchID)
}
