package middleware

import (
	"net/http"
	"slices"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
)

// GetTwitchPlatformID returns the Twitch platform user ID from a user's connections, or empty string if not found.
func GetTwitchPlatformID(user *common.User) string {
	for _, conn := range user.Connections {
		if conn.Platform == common.TWITCH {
			return conn.UserID
		}
	}

	return ""
}

// IsChannelAuthorized returns true if the user is an admin, the broadcaster of the given channel,
// or a channel ambassador. It is used as a shared auth check across channel-scoped write routes.
func IsChannelAuthorized(
	request *http.Request,
	user *common.User,
	channelID string,
	postgres *db.PostgresClient,
) bool {
	if user.Level >= int(common.ADMIN) {
		return true
	}

	twitchID := GetTwitchPlatformID(user)

	if twitchID == channelID {
		return true
	}

	ambassadors, err := postgres.GetChannelAmbassadors(request.Context(), channelID, common.TWITCH)
	if err != nil {
		return false
	}

	return slices.Contains(ambassadors, twitchID)
}
