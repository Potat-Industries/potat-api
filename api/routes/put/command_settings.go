// Package put contains routes for http.MethodPut requests.
package put

import (
	"encoding/json"
	"net/http"
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
		Method:  http.MethodPut,
		Handler: putCommandSettings,
		UseAuth: true,
	})
}

func putCommandSettings(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop
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

	var input common.CommandSettings
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Invalid request body"}},
		}, start)

		return
	}

	channelID := input.ChannelID
	if channelID == "" {
		channelID = request.URL.Query().Get("id")
	}

	if channelID == "" {
		for _, conn := range user.Connections {
			if conn.Platform == common.TWITCH {
				channelID = conn.UserID

				break
			}
		}
	}

	if channelID == "" || input.Command == "" {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "channel_id and command are required"}},
		}, start)

		return
	}

	input.ChannelID = channelID

	if !middleware.IsChannelAuthorized(request, user, channelID, postgres) {
		api.GenericResponse(writer, http.StatusForbidden, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Forbidden"}},
		}, start)

		return
	}

	if err := postgres.UpsertCommandSettings(request.Context(), input); err != nil {
		logger.Error.Printf("Error upserting command settings: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[any]{
			Errors: &[]common.ErrorMessage{{Message: "Failed to update command settings"}},
		}, start)

		return
	}

	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[any]{
		Data: &[]any{},
	}, start)
}
