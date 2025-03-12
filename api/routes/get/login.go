package get

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"potat-api/api"
	"potat-api/common"
	"potat-api/common/utils"
)

const (
	twitchOauthURI   = "https://id.twitch.tv/oauth2/authorize"
	twitchOauthToken = "https://id.twitch.tv/oauth2/token"
	// TODO: load from config.
	scopes = "channel:bot chat:read user:read:moderated_channels channel:manage:broadcast channel:manage:redemptions channel:read:subscriptions moderator:read:followers channel:read:hype_train channel:read:guest_star"
)

var replyDeny sync.Map

func init() {
	api.SetRoute(api.Route{
		Path:    "/login",
		Method:  http.MethodGet,
		Handler: twitchLoginHandler,
	}, false)
}

func setReplyDeny() string {
	nonce := strconv.Itoa(rand.Int())
	replyDeny.Store(nonce, true)
	go func(n string) {
		time.Sleep(20 * time.Second)
		replyDeny.Delete(n)
	}(nonce)

	return nonce
}

func twitchLoginHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	config := utils.LoadConfig()

	defer func() {
		if r := recover(); r != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}()

	query := r.URL.Query()
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
		http.Redirect(w, r, uri, http.StatusFound)

		return
	}

	// Disallow replay attacks
	if _, ok := replyDeny.Load(state); !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)

		return
	}
	replyDeny.Delete(state)

	// Excahnge code for access token
	tokenResp, err := http.PostForm(
		twitchOauthToken,
		url.Values{
			"client_id":     {config.Twitch.ClientID},
			"client_secret": {config.Twitch.ClientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {redirectURI},
		},
	)
	if err != nil {
		http.Error(w, "Failed to get access token", http.StatusInternalServerError)

		return
	}
	defer tokenResp.Body.Close()

	var tokenData common.GenericOAUTHResponse
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		http.Error(w, "Failed to parse token response", http.StatusInternalServerError)

		return
	}

	ok, validation, err := utils.ValidateHelixToken(
		tokenData.AccessToken,
		false,
	)
	if err != nil || !ok || validation.UserID == "" {
		api.GenericResponse(w, http.StatusUnauthorized, AuthorizedUserResponse{
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
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(html))
	if err != nil {
		utils.Warn.Println("Failed to write document: ", err)
	}
}
