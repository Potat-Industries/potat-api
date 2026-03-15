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

// ChannelsResponse is the response type for the GET /channels endpoint.
type ChannelsResponse = common.GenericResponse[common.ChannelListItem]

func init() {
	api.SetRoute(api.Route{
		Path:    "/channels",
		Method:  http.MethodGet,
		Handler: getChannelsHandler,
		UseAuth: false,
	})
}

func getChannelsHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	postgres, ok := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")
		api.GenericResponse(writer, http.StatusInternalServerError, ChannelsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)

		return
	}

	channels, err := postgres.GetAllChannels(request.Context())
	if err != nil {
		logger.Error.Printf("Error fetching channels: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, ChannelsResponse{
			Errors: &[]common.ErrorMessage{{Message: "Failed to fetch channels"}},
		}, start)

		return
	}

	if channels == nil {
		channels = []common.ChannelListItem{}
	}

	api.GenericResponse(writer, http.StatusOK, ChannelsResponse{
		Data: &channels,
	}, start)
}
