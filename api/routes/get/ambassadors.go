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
)

// AmbassadorsResponse is the response type for GET /channel/ambassadors.
type AmbassadorsResponse = common.GenericResponse[string]

func init() {
	api.SetRoute(api.Route{
		Path:    "/channel/ambassadors",
		Method:  http.MethodGet,
		Handler: getAmbassadorsHandler,
		UseAuth: true,
	})
}

func getAmbassadorsHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := request.Context().Value(middleware.AuthedUser).(*common.User)
	if !ok || user == nil {
		api.GenericResponse(writer, http.StatusUnauthorized, AmbassadorsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	postgres, ok := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		api.GenericResponse(writer, http.StatusInternalServerError, AmbassadorsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)

		return
	}

	channelID := resolveChannelID(request, user)
	if channelID == "" {
		api.GenericResponse(writer, http.StatusBadRequest, AmbassadorsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Could not resolve channel ID"}},
		}, start)

		return
	}

	if !middleware.IsChannelAuthorized(request, user, channelID, postgres) {
		api.GenericResponse(writer, http.StatusForbidden, AmbassadorsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Forbidden"}},
		}, start)

		return
	}

	ambassadors, err := postgres.GetChannelAmbassadors(request.Context(), channelID, common.TWITCH)
	if err != nil {
		logger.Error.Printf("Error fetching ambassadors: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, AmbassadorsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Failed to fetch ambassadors"}},
		}, start)

		return
	}

	if ambassadors == nil {
		ambassadors = []string{}
	}

	api.GenericResponse(writer, http.StatusOK, AmbassadorsResponse{
		Data: &ambassadors,
	}, start)
}
