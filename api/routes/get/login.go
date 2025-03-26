// Package get contains routes for http.MethodGet requests.
package get

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"potat-api/api"
	"potat-api/common"
	"potat-api/common/logger"
	"potat-api/common/utils"
)

//nolint:gosec,lll
const (
	twitchOauthURI   = "https://id.twitch.tv/oauth2/authorize"
	twitchOauthToken = "https://id.twitch.tv/oauth2/token"
	scopes           = "channel:bot chat:read user:read:moderated_channels channel:manage:broadcast channel:manage:redemptions channel:read:subscriptions moderator:read:followers channel:read:hype_train channel:read:guest_star"
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
		time.Sleep(20 * time.Second)
		replyDeny.Delete(n)
	}(nonce)

	return nonce
}

func handleErr(w http.ResponseWriter, start time.Time) {
	if r := recover(); r != nil {
		api.GenericResponse(w, http.StatusInternalServerError, common.GenericResponse[any]{
			Data:   nil,
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)
	}
}

func twitchLoginHandler(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop
	start := time.Now()

	config := utils.LoadConfig()

	defer handleErr(writer, start)

	query := request.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	redirectURI := fmt.Sprintf("%slogin", config.Twitch.OauthURI)

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
		Timeout: 10 * time.Second,
	}

	// Excahnge code for access token
	tokenResp, err := client.Do(req)
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
		false,
	)
	if err != nil || !ok || validation.UserID == "" {
		api.GenericResponse(writer, http.StatusUnauthorized, AuthorizedUserResponse{
			Data:   &[]SiteUserData{},
			Errors: &[]common.ErrorMessage{{Message: "Failed to validate access token"}},
		}, start)

		return
	}

	// check if user exists in the database
	// user, err := db.Postgres.GetUserByConnection(
	// 	r.Context(),
	// 	validation.UserID,
	// 	common.TWITCH,
	// )
	// if not send rabbitmq message to create user on potat
	// if user created update data, set oauth token

	// auth successful, close the popup and send token backarino
	html := fmt.Sprintf(`
		<script>
			if (window.opener) {
				window.opener.postMessage(%s, '%s');
				window.close();
			}
		</script>
		`,
		string("token and stuff lol"),
		strings.Replace(config.Twitch.OauthURI, "api.", "", 1),
	)
	writer.Header().Set("Content-Type", "text/html")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write([]byte(html))
	if err != nil {
		logger.Warn.Println("Failed to write document: ", err)
	}
}
