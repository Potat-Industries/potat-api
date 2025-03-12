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
	"potat-api/common/utils"
)

type SiteUserData struct {
	Name      string `json:"name"`
	TwitchPFP string `json:"twitch_pfp"`
	StvPFP    string `json:"stv_pfp"`
	ChatColor string `json:"chatColor"`
	UserPaint string `json:"userPaint"`
	JoinState string `json:"join_state"`
}

type AuthorizedUserResponse = common.GenericResponse[SiteUserData]

func init() {
	api.SetRoute(api.Route{
		Path:    "/twitch/me",
		Method:  http.MethodGet,
		Handler: getAuthenticatedUser,
	}, true)
}

func getChannelState(ctx context.Context, channelID string, platform common.Platforms) string {
	channelData, err := db.Postgres.GetChannelByID(ctx, channelID, platform)
	if err != nil {
		return "NEVER"
	}

	return channelData.State
}

func getAuthenticatedUser(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	userData, ok := r.Context().Value(middleware.AuthedUser).(*common.User)
	if !ok || userData == nil {
		api.GenericResponse(w, http.StatusUnauthorized, AuthorizedUserResponse{
			Data:   &[]SiteUserData{},
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	// Check userconnections length
	if len(userData.Connections) == 0 {
		api.GenericResponse(w, http.StatusUnauthorized, AuthorizedUserResponse{
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
		api.GenericResponse(w, http.StatusUnauthorized, AuthorizedUserResponse{
			Data:   &[]SiteUserData{},
			Errors: &[]common.ErrorMessage{{Message: "User Twitch connection not found"}},
		}, start)

		return
	}

	var stvMeta common.StvUserMeta
	err := json.Unmarshal(stvConnection.Meta, &stvMeta)
	if err != nil {
		utils.Error.Println("Error unmarshalling stv meta: ", err)
	}

	var twitchMeta common.TwitchUserMeta
	err = json.Unmarshal(twitchConnection.Meta, &twitchMeta)
	if err != nil {
		utils.Error.Println("Error unmarshalling twitch meta: ", err)
	}

	user := SiteUserData{
		Name:      userData.Display,
		TwitchPFP: twitchConnection.PFP,
		StvPFP:    stvConnection.PFP,
		ChatColor: twitchMeta.Color,
		UserPaint: stvMeta.PaintID,
		JoinState: getChannelState(r.Context(), twitchConnection.UserID, common.TWITCH),
	}

	api.GenericResponse(w, http.StatusOK, AuthorizedUserResponse{
		Data: &[]SiteUserData{user},
	}, start)
}
