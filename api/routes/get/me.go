// Package get contains routes for http.MethodGet requests.
package get

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"potat-api/api"
	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/logger"
)

// SiteUserData represents the user data returned by the /twitch/me endpoint.
type SiteUserData struct {
	Name      string `json:"name"`
	TwitchPFP string `json:"twitch_pfp"`
	StvPFP    string `json:"stv_pfp"`
	ChatColor string `json:"chatColor"`
	UserPaint string `json:"userPaint"`
	JoinState string `json:"join_state"`
}

// AuthorizedUserResponse is the response type for the /twitch/me endpoint.
type AuthorizedUserResponse = common.GenericResponse[SiteUserData]

func init() {
	api.SetRoute(api.Route{
		Path:    "/twitch/me",
		Method:  http.MethodGet,
		Handler: getAuthenticatedUser,
		UseAuth: true,
	})
}

func getChannelState(ctx context.Context, channelID string, platform common.Platforms) string {
	postgres, ok := ctx.Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")

		return "NEVER"
	}

	channelData, err := postgres.GetChannelByID(ctx, channelID, platform)
	if err != nil {
		return "NEVER"
	}

	return channelData.State
}

func getAuthenticatedUser(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	userData, ok := request.Context().Value(middleware.AuthedUser).(*common.User)
	if !ok || userData == nil {
		api.GenericResponse(writer, http.StatusUnauthorized, AuthorizedUserResponse{
			Data:   &[]SiteUserData{},
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	// Check userconnections length
	if len(userData.Connections) == 0 {
		api.GenericResponse(writer, http.StatusUnauthorized, AuthorizedUserResponse{
			Data:   &[]SiteUserData{},
			Errors: &[]common.ErrorMessage{{Message: "User connections not found"}},
		}, start)

		return
	}

	var stvConnection common.UserConnection
	var twitchConnection common.UserConnection
	for _, connection := range userData.Connections {
		switch connection.Platform {
		case "STV":
			stvConnection = connection
		case "TWITCH":
			twitchConnection = connection
		default:
			continue
		}
	}

	if twitchConnection.UserID == "" {
		api.GenericResponse(writer, http.StatusUnauthorized, AuthorizedUserResponse{
			Data:   &[]SiteUserData{},
			Errors: &[]common.ErrorMessage{{Message: "User Twitch connection not found"}},
		}, start)

		return
	}

	twitchMeta, stvMeta := parseMetadata(twitchConnection.Meta, stvConnection.Meta)

	user := SiteUserData{
		Name:      userData.Display,
		TwitchPFP: twitchConnection.PFP,
		StvPFP:    stvConnection.PFP,
		ChatColor: twitchMeta.Color,
		UserPaint: stvMeta.PaintID,
		JoinState: getChannelState(request.Context(), twitchConnection.UserID, common.TWITCH),
	}

	api.GenericResponse(writer, http.StatusOK, AuthorizedUserResponse{
		Data: &[]SiteUserData{user},
	}, start)
}

func parseMetadata(twitchMeta, stvMeta common.UserMeta) (common.TwitchUserMeta, common.StvUserMeta) {
	twitchData := common.TwitchUserMeta{}
	stvData := common.StvUserMeta{}

	if err := json.Unmarshal([]byte(twitchMeta), &twitchData); err != nil {
		logger.Error.Printf("Error parsing Twitch metadata: %v", err)
	}

	if err := json.Unmarshal([]byte(stvMeta), &stvData); err != nil {
		logger.Error.Printf("Error parsing STV metadata: %v", err)
	}

	return twitchData, stvData
}
