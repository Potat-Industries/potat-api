// Package patch contains routes for http.MethodPatch requests.
package patch

import (
	"encoding/json"
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
		Path:    "/users/me/settings",
		Method:  http.MethodPatch,
		Handler: patchUserSettings,
		UseAuth: true,
	})
	api.SetRoute(api.Route{
		Path:    "/channels/me/settings",
		Method:  http.MethodPatch,
		Handler: patchChannelSettings,
		UseAuth: true,
	})
}

func patchUserSettings(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := request.Context().Value(middleware.AuthedUser).(*common.User)
	if !ok || user == nil {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	var input common.UserSettings
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Invalid request body"}},
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

	if err := postgres.UpdateUserSettings(request.Context(), user.ID, input); err != nil {
		logger.Error.Printf("Error updating user settings: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Failed to update settings"}},
		}, start)

		return
	}

	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[any]{
		Data: &[]any{},
	}, start)
}

func patchChannelSettings(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop
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

	// Resolve channel ID from ?id= or fall back to the user's own Twitch channel ID.
	channelID := request.URL.Query().Get("id")
	if channelID == "" {
		for _, conn := range user.Connections {
			if conn.Platform == common.TWITCH {
				channelID = conn.UserID

				break
			}
		}
	}

	if channelID == "" {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "channel id is required"}},
		}, start)

		return
	}

	// Only the broadcaster or an ambassador (or admin) may update settings.
	var twitchID string
	for _, conn := range user.Connections {
		if conn.Platform == common.TWITCH {
			twitchID = conn.UserID

			break
		}
	}

	if twitchID != channelID && user.Level < int(common.ADMIN) {
		ambassadors, err := postgres.GetChannelAmbassadors(request.Context(), channelID, common.TWITCH)
		if err != nil || !slices.Contains(ambassadors, twitchID) {
			api.GenericResponse(writer, http.StatusForbidden, common.GenericResponse[any]{
				Errors: &[]common.ErrorMessage{{Message: "Forbidden"}},
			}, start)

			return
		}
	}

	var input common.ChannelSettings
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Invalid request body"}},
		}, start)

		return
	}

	if err := postgres.UpdateChannelSettings(request.Context(), channelID, input); err != nil {
		logger.Error.Printf("Error updating channel settings: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Failed to update settings"}},
		}, start)

		return
	}

	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[any]{
		Data: &[]any{},
	}, start)
}
