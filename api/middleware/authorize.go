package middleware

import (
	"net/http"
	"slices"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
)

// GetPlatformID returns the platform user ID for the given platform from a user's connections,
// or empty string if not found.
func GetPlatformID(user *common.User, platform common.Platforms) string {
	for _, conn := range user.Connections {
		if conn.Platform == platform {
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
	platform common.Platforms,
	postgres *db.PostgresClient,
) bool {
	if user.Level >= int(common.ADMIN) {
		return true
	}

	platformID := GetPlatformID(user, platform)

	if platformID == channelID {
		return true
	}

	ambassadors, err := postgres.GetChannelAmbassadors(request.Context(), channelID, platform)
	if err != nil {
		return false
	}

	return slices.Contains(ambassadors, platformID)
}
