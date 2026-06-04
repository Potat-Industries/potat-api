// Package get contains routes for http.MethodGet requests.
package get

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Potat-Industries/potat-api/api"
	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/Potat-Industries/potat-api/common/utils"
	"github.com/google/uuid"
)

//nolint:gosec,lll
const (
	twitchOauthURI    = "https://id.twitch.tv/oauth2/authorize"
	twitchOauthToken  = "https://id.twitch.tv/oauth2/token"
	scopes            = "bits:read chat:read chat:edit user:read:emotes user:read:moderated_channels channel:manage:broadcast channel:manage:redemptions channel:manage:ads channel:edit:commercial channel:read:subscriptions channel:read:guest_star channel:read:hype_train channel:read:charity channel:read:goals channel:read:polls channel:read:vips channel:read:predictions channel:moderate channel:bot moderator:read:followers moderator:read:chatters moderator:read:automod_settings moderator:read:blocked_terms moderator:read:chat_settings moderator:read:shield_mode"
	replyDenyTTL      = 20 * time.Second
	httpClientTimeout = 10 * time.Second
)

var replyDeny sync.Map //nolint:gochecknoglobals // Used to prevent replay attacks on the oauth flow.

func init() {
	api.SetRoute(api.Route{
		Path:    "/login",
		Method:  http.MethodGet,
		Handler: twitchLoginHandler,
		UseAuth: false,
	})
}

func setReplyDeny() string {
	nonce := uuid.New().String()
	replyDeny.Store(nonce, true)
	go func(n string) {
		time.Sleep(replyDenyTTL)
		replyDeny.Delete(n)
	}(nonce)

	return nonce
}

func loginPostMessageOrigin(oauthURI string) string {
	parsed, err := url.Parse(oauthURI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.Replace(strings.TrimRight(oauthURI, "/"), "api.", "", 1)
	}

	origin := url.URL{
		Scheme: parsed.Scheme,
		Host:   strings.TrimPrefix(parsed.Host, "api."),
	}

	return origin.String()
}

func handleErr(w http.ResponseWriter, start time.Time) {
	if r := recover(); r != nil {
		api.GenericResponse(w, http.StatusInternalServerError, common.GenericResponse[any]{
			Data:   nil,
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)
	}
}

func twitchLoginHandler(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop,maintidx
	start := time.Now()

	config := utils.LoadConfig()

	defer handleErr(writer, start)

	query := request.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	redirectURI := strings.TrimRight(config.Twitch.OauthURI, "/") + "/login"

	// Redirect to twitch oauth
	if code == "" {
		params := url.Values{
			"client_id":     {config.Twitch.ClientID},
			"force_verify":  {"false"},
			"redirect_uri":  {redirectURI},
			"response_type": {"code"},
			"scope":         {scopes},
			"state":         {setReplyDeny()},
		}.Encode()
		uri := fmt.Sprintf("%s?%s", twitchOauthURI, params)
		http.Redirect(writer, request, uri, http.StatusFound)

		return
	}

	// Disallow replay attacks
	if _, ok := replyDeny.Load(state); !ok {
		http.Error(writer, "Forbidden", http.StatusForbidden)

		return
	}
	replyDeny.Delete(state)

	data := url.Values{
		"client_id":     {config.Twitch.ClientID},
		"client_secret": {config.Twitch.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(
		request.Context(),
		http.MethodPost,
		twitchOauthToken,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		http.Error(writer, "Failed to create request", http.StatusInternalServerError)

		return
	}

	client := &http.Client{
		Timeout: httpClientTimeout,
	}

	// Excahnge code for access token
	tokenResp, err := client.Do(req) //nolint:gosec
	if err != nil {
		http.Error(writer, "Failed to get access token", http.StatusInternalServerError)

		return
	}
	defer func() {
		if err = tokenResp.Body.Close(); err != nil {
			logger.Error.Println("Failed to close response body: ", err)
		}
	}()

	var tokenData common.GenericOAUTHResponse
	if err = json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		http.Error(writer, "Failed to parse token response", http.StatusInternalServerError)

		return
	}

	ok, validation, err := utils.ValidateHelixToken(
		request.Context(),
		tokenData.AccessToken,
		true, // returnAll = true to get login + user_id
	)
	if err != nil || !ok {
		api.GenericResponse(writer, http.StatusUnauthorized, AuthorizedUserResponse{
			Data:   &[]SiteUserData{},
			Errors: &[]common.ErrorMessage{{Message: "Failed to validate access token"}},
		}, start)

		return
	}

	postgres, ok := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	// Upsert OAuth token (non fatal)
	if _, upsertErr := postgres.Exec(
		request.Context(),
		`INSERT INTO connection_oauth (platform_id, access_token, refresh_token, scope, expires_in, added_at, platform)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (platform_id, platform) DO UPDATE SET
		     access_token = EXCLUDED.access_token,
		     refresh_token = EXCLUDED.refresh_token,
		     scope = EXCLUDED.scope,
		     expires_in = EXCLUDED.expires_in,
		     added_at = EXCLUDED.added_at`,
		validation.UserID,
		tokenData.AccessToken,
		tokenData.RefreshToken,
		tokenData.Scope,
		tokenData.ExpiresIn,
		time.Now(),
		common.TWITCH,
	); upsertErr != nil {
		logger.Warn.Println("Failed to upsert OAuth token: ", upsertErr)
	}

	user, err := postgres.GetUserByName(request.Context(), validation.Login)
	if err != nil {
		if errors.Is(err, db.ErrPostgresNoRows) {
			api.GenericResponse(writer, http.StatusNotFound, AuthorizedUserResponse{
				Data:   &[]SiteUserData{},
				Errors: &[]common.ErrorMessage{{Message: "User not found. Please make sure the bot is in your channel first."}},
			}, start)
		} else {
			logger.Error.Println("Error fetching user: ", err)
			http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		}

		return
	}

	var pfp, stvID string
	for _, conn := range user.Connections {
		switch conn.Platform {
		case common.TWITCH:
			pfp = conn.PFP
		case common.STV:
			stvID = conn.UserID
		case common.DISCORD, common.KICK:
			// not used for login payload
		}
	}

	channelData, channelErr := postgres.GetChannelByID(request.Context(), validation.UserID, common.TWITCH)
	isChannel := channelErr == nil && channelData != nil && channelData.State == "JOINED"

	auth := middleware.NewAuthenticator(config.Twitch.ClientSecret, nil)
	jwtToken, err := auth.CreateJWT(user.ID)
	if err != nil {
		logger.Error.Println("Failed to create JWT: ", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	type loginPayload struct {
		Token     string `json:"token"`
		ID        string `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		StvID     string `json:"stv_id"` //nolint:tagliatelle // API contract uses snake_case
		PFP       string `json:"pfp"`
		IsChannel bool   `json:"is_channel"` //nolint:tagliatelle // API contract uses snake_case
	}

	payloadJSON, err := json.Marshal(loginPayload{
		Token:     jwtToken,
		ID:        validation.UserID,
		Login:     validation.Login,
		Name:      user.Display,
		StvID:     stvID,
		PFP:       pfp,
		IsChannel: isChannel,
	})
	if err != nil {
		logger.Error.Println("Failed to marshal login payload: ", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	targetOriginJSON, err := json.Marshal(loginPostMessageOrigin(config.Twitch.OauthURI))
	if err != nil {
		logger.Error.Println("Failed to marshal login target origin: ", err)
		http.Error(writer, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	html := fmt.Sprintf(`
		<script>
			if (window.opener) {
				window.opener.postMessage(%s, %s);
				window.close();
			}
		</script>
		`,
		string(payloadJSON),
		string(targetOriginJSON),
	)
	writer.Header().Set("Content-Type", "text/html")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write([]byte(html))
	if err != nil {
		logger.Warn.Println("Failed to write document: ", err)
	}
}
