// Package get contains routes for http.MethodGet requests.
package get

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
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

//nolint:gosec
const (
	discordAuthorizeURL = "https://discord.com/api/oauth2/authorize"
	discordTokenURL     = "https://discord.com/api/oauth2/token"
	discordMeURL        = "https://discord.com/api/users/@me"

	spotifyAuthorizeURL = "https://accounts.spotify.com/authorize"
	spotifyTokenURL     = "https://accounts.spotify.com/api/token"
	spotifyMeURL        = "https://api.spotify.com/v1/me"
	spotifyScopes       = "streaming user-read-recently-played user-top-read " +
		"user-read-playback-position user-read-playback-state " +
		"user-read-currently-playing user-modify-playback-state user-follow-read"

	kickAuthorizeURL = "https://id.kick.com/oauth/authorize"
	kickTokenURL     = "https://id.kick.com/oauth/token"
	kickScopes       = "user:read channel:read channel:write chat:write events:subscribe"

	anilistAuthorizeURL = "https://anilist.co/api/v2/oauth/authorize"
	anilistTokenURL     = "https://anilist.co/api/v2/oauth/token"
	anilistGQLURL       = "https://graphql.anilist.co"

	traktAuthorizeURL = "https://api.trakt.tv/oauth/authorize"
	traktTokenURL     = "https://api.trakt.tv/oauth/token"
	traktMeURL        = "https://api.trakt.tv/users/me"

	ffzAuthorizeURL = "https://api.frankerfacez.com/auth/authorize"
	ffzTokenURL     = "https://api.frankerfacez.com/auth/token"
	ffzScopes       = "collection_edit"

	steamOpenIDURL    = "https://steamcommunity.com/openid/login"
	steamOpenIDNS     = "http://specs.openid.net/auth/2.0"
	steamFriendOffset = int64(76561197960265728)

	oauthStateTTL    = 60 * time.Second
	oauthHTTPTimeout = 10 * time.Second
)

// oauthStates maps state strings to true with a TTL for replay-attack prevention.
var oauthStates sync.Map //nolint:gochecknoglobals

// kickPKCEVerifiers maps OAuth state strings to PKCE code verifiers, with the same TTL as oauthStates.
var kickPKCEVerifiers sync.Map //nolint:gochecknoglobals

func newOAuthState(userID int) string {
	nonce := uuid.New().String()
	state := fmt.Sprintf("%s:%d", nonce, userID)
	oauthStates.Store(state, true)
	time.AfterFunc(oauthStateTTL, func() {
		oauthStates.Delete(state)
	})

	return state
}

// newKickOAuthState creates an OAuth state and associated PKCE code verifier.
// State format: {nonce}:{userID}.
// The PKCE code verifier is stored server-side and must be retrieved on callback.
func newKickOAuthState(userID int) (state, codeVerifier, codeChallenge string) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		codeVerifier = uuid.New().String()
	} else {
		codeVerifier = hex.EncodeToString(buf)
	}

	sum := sha256.Sum256([]byte(codeVerifier))
	codeChallenge = base64.RawURLEncoding.EncodeToString(sum[:])

	nonce := uuid.New().String()
	state = fmt.Sprintf("%s:%d", nonce, userID)

	// Store replay-protection state and PKCE verifier with the same TTL.
	oauthStates.Store(state, true)
	kickPKCEVerifiers.Store(state, codeVerifier)
	time.AfterFunc(oauthStateTTL, func() {
		oauthStates.Delete(state)
		kickPKCEVerifiers.Delete(state)
	})

	return state, codeVerifier, codeChallenge
}

// consumeOAuthState validates a state string, deletes it, and returns the embedded user ID.
func consumeOAuthState(state string) (int, bool) {
	if _, ok := oauthStates.LoadAndDelete(state); !ok {
		return 0, false
	}

	parts := strings.SplitN(state, ":", 3) //nolint:mnd
	if len(parts) < 2 {                    //nolint:mnd
		return 0, false
	}

	var userID int
	if _, err := fmt.Sscanf(parts[1], "%d", &userID); err != nil {
		return 0, false
	}

	return userID, true
}

// extractKickCodeVerifier retrieves the server-side PKCE code verifier for a Kick state.
func extractKickCodeVerifier(state string) string {
	v, ok := kickPKCEVerifiers.Load(state)
	if !ok {
		return ""
	}

	s, _ := v.(string)

	return s
}

// authUser retrieves the authenticated user from the request context.
func authUser(request *http.Request) (*common.User, bool) {
	user, ok := request.Context().Value(middleware.AuthedUser).(*common.User)

	return user, ok && user != nil
}

// getTwitchPlatformID finds the Twitch platform ID from a user's connections.
func getTwitchPlatformID(user *common.User) string {
	for _, conn := range user.Connections {
		if conn.Platform == common.TWITCH {
			return conn.UserID
		}
	}

	return ""
}

// oauthPostMessage builds the postMessage HTML used to close popups.
func oauthPostMessage(payload map[string]any) string {
	data, _ := json.Marshal(payload) //nolint:errchkjson

	return fmt.Sprintf(`<script>
(function () {
  try {
    if (window.opener && window.opener.location && window.opener.location.origin) {
      window.opener.postMessage(%s, window.opener.location.origin);
    }
  } catch (e) {
    // Ignore errors and just close the window.
  } finally {
    window.close();
  }
})();
</script>`, string(data))
}

// oauthErrorHTML builds a minimal error postMessage HTML.
func oauthErrorHTML(message string) string {
	return oauthPostMessage(map[string]any{"error": message})
}

// oauthSuccessHTML builds a minimal success postMessage HTML.
func oauthSuccessHTML(platform string) string {
	return oauthPostMessage(map[string]any{"platform": platform, "ok": true})
}

// sendHTML writes an HTML response.
func sendHTML(writer http.ResponseWriter, code int, body string) {
	writer.Header().Set("Content-Type", "text/html")
	writer.WriteHeader(code)

	if _, err := writer.Write([]byte(body)); err != nil {
		logger.Warn.Println("Failed to write HTML response:", err)
	}
}

// ---------------------------------------------------------------------------
// init — register all auth routes
// ---------------------------------------------------------------------------

func init() { //nolint:funlen
	// Discord
	api.SetRoute(api.Route{
		Path: "/auth/discord/authorize", Method: http.MethodGet, Handler: discordAuthorizeHandler, UseAuth: true,
	})
	api.SetRoute(api.Route{Path: "/auth/discord", Method: http.MethodGet, Handler: discordCallbackHandler, UseAuth: false})
	api.SetRoute(api.Route{
		Path: "/auth/discord/join", Method: http.MethodGet, Handler: discordJoinHandler, UseAuth: false,
	})

	// Spotify
	api.SetRoute(api.Route{
		Path: "/auth/spotify/authorize", Method: http.MethodGet, Handler: spotifyAuthorizeHandler, UseAuth: true,
	})
	api.SetRoute(api.Route{Path: "/auth/spotify", Method: http.MethodGet, Handler: spotifyCallbackHandler, UseAuth: false})

	// Kick
	api.SetRoute(api.Route{
		Path: "/auth/kick/authorize", Method: http.MethodGet, Handler: kickAuthorizeHandler, UseAuth: true,
	})
	api.SetRoute(api.Route{Path: "/auth/kick", Method: http.MethodGet, Handler: kickCallbackHandler, UseAuth: false})

	// Anilist
	api.SetRoute(api.Route{
		Path: "/auth/anilist/authorize", Method: http.MethodGet, Handler: anilistAuthorizeHandler, UseAuth: true,
	})
	api.SetRoute(api.Route{Path: "/auth/anilist", Method: http.MethodGet, Handler: anilistCallbackHandler, UseAuth: false})

	// Trakt
	api.SetRoute(api.Route{
		Path: "/auth/trakt/authorize", Method: http.MethodGet, Handler: traktAuthorizeHandler, UseAuth: true,
	})
	api.SetRoute(api.Route{Path: "/auth/trakt", Method: http.MethodGet, Handler: traktCallbackHandler, UseAuth: false})

	// FFZ — public, direct redirect
	api.SetRoute(api.Route{
		Path: "/auth/ffz/authorize", Method: http.MethodGet, Handler: ffzAuthorizeHandler, UseAuth: false,
	})
	api.SetRoute(api.Route{Path: "/auth/ffz", Method: http.MethodGet, Handler: ffzCallbackHandler, UseAuth: false})

	// Steam — OpenID 2.0
	api.SetRoute(api.Route{
		Path: "/auth/steam/authorize", Method: http.MethodGet, Handler: steamAuthorizeHandler, UseAuth: true,
	})
	api.SetRoute(api.Route{Path: "/auth/steam", Method: http.MethodGet, Handler: steamCallbackHandler, UseAuth: false})
}

// ---------------------------------------------------------------------------
// Discord
// ---------------------------------------------------------------------------

func discordAuthorizeHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := authUser(request)
	if !ok {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[string]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	config := utils.LoadConfig()
	state := newOAuthState(user.ID)
	redirectURI := strings.TrimRight(config.Discord.OAuthURI, "/") + "/auth/discord"

	params := url.Values{
		"client_id":     {config.Discord.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"identify"},
		"state":         {state},
	}

	target := fmt.Sprintf("%s?%s", discordAuthorizeURL, params.Encode())
	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[string]{
		Data: &[]string{target},
	}, start)
}

func discordCallbackHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	userID, ok := consumeOAuthState(state)
	if !ok {
		sendHTML(writer, http.StatusForbidden, oauthErrorHTML("Invalid or expired state"))

		return
	}

	config := utils.LoadConfig()
	redirectURI := strings.TrimRight(config.Discord.OAuthURI, "/") + "/auth/discord"

	tok, err := exchangeFormToken(request.Context(), discordTokenURL, url.Values{
		"client_id":     {config.Discord.ClientID},
		"client_secret": {config.Discord.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		logger.Error.Println("Discord token exchange failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Token exchange failed"))

		return
	}

	discordID, err := fetchDiscordUserID(request.Context(), tok.AccessToken)
	if err != nil {
		logger.Error.Println("Discord user fetch failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Failed to fetch Discord user"))

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Internal Server Error"))

		return
	}

	_ = userID // stored in state; could be used to link to internal user row

	scope := strings.Fields(tok.Scope)
	if err := postgres.UpsertOAuthToken(
		request.Context(), discordID, common.DISCORD,
		tok.AccessToken, tok.RefreshToken, scope, tok.ExpiresIn,
	); err != nil {
		logger.Warn.Println("Failed to upsert Discord OAuth token:", err)
	}

	sendHTML(writer, http.StatusOK, oauthSuccessHTML("discord"))
}

func discordJoinHandler(writer http.ResponseWriter, request *http.Request) {
	config := utils.LoadConfig()

	params := url.Values{
		"client_id":   {config.Discord.ClientID},
		"permissions": {"414464674880"},
		"scope":       {"bot identify messages.read guilds.members.read"},
	}

	target := fmt.Sprintf("https://discord.com/oauth2/authorize?%s", params.Encode())
	http.Redirect(writer, request, target, http.StatusFound)
}

// ---------------------------------------------------------------------------
// Spotify
// ---------------------------------------------------------------------------

func spotifyAuthorizeHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := authUser(request)
	if !ok {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[string]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	config := utils.LoadConfig()
	state := newOAuthState(user.ID)
	redirectURI := strings.TrimRight(config.Spotify.OAuthURI, "/") + "/auth/spotify"

	params := url.Values{
		"client_id":     {config.Spotify.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {spotifyScopes},
		"state":         {state},
	}

	target := fmt.Sprintf("%s?%s", spotifyAuthorizeURL, params.Encode())
	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[string]{
		Data: &[]string{target},
	}, start)
}

func spotifyCallbackHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	userID, ok := consumeOAuthState(state)
	if !ok {
		sendHTML(writer, http.StatusForbidden, oauthErrorHTML("Invalid or expired state"))

		return
	}

	config := utils.LoadConfig()
	redirectURI := strings.TrimRight(config.Spotify.OAuthURI, "/") + "/auth/spotify"

	tok, err := exchangeFormToken(request.Context(), spotifyTokenURL, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}, withBasicAuth(config.Spotify.ClientID, config.Spotify.ClientSecret))
	if err != nil {
		logger.Error.Println("Spotify token exchange failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Token exchange failed"))

		return
	}

	spotifyID, err := fetchJSONField(
		request.Context(), spotifyMeURL,
		map[string]string{"Authorization": "Bearer " + tok.AccessToken},
		"id",
	)
	if err != nil {
		logger.Error.Println("Spotify me fetch failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Failed to fetch Spotify user"))

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK || userID == 0 {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Internal Server Error"))

		return
	}

	scope := strings.Fields(tok.Scope)
	if err := postgres.UpsertOAuthToken(
		request.Context(), spotifyID, common.SPOTIFY,
		tok.AccessToken, tok.RefreshToken, scope, tok.ExpiresIn,
	); err != nil {
		logger.Warn.Println("Failed to upsert Spotify OAuth token:", err)
	}

	sendHTML(writer, http.StatusOK, oauthSuccessHTML("spotify"))
}

// ---------------------------------------------------------------------------
// Kick
// ---------------------------------------------------------------------------

func kickAuthorizeHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := authUser(request)
	if !ok {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[string]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	config := utils.LoadConfig()
	state, _, codeChallenge := newKickOAuthState(user.ID)
	redirectURI := strings.TrimRight(config.Kick.OAuthURI, "/") + "/auth/kick"

	params := url.Values{
		"client_id":             {config.Kick.ClientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {kickScopes},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	target := fmt.Sprintf("%s?%s", kickAuthorizeURL, params.Encode())
	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[string]{
		Data: &[]string{target},
	}, start)
}

func kickCallbackHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	userID, ok := consumeOAuthState(state)
	if !ok {
		sendHTML(writer, http.StatusForbidden, oauthErrorHTML("Invalid or expired state"))

		return
	}

	codeVerifier := extractKickCodeVerifier(state)

	config := utils.LoadConfig()
	redirectURI := strings.TrimRight(config.Kick.OAuthURI, "/") + "/auth/kick"

	tok, err := exchangeFormToken(request.Context(), kickTokenURL, url.Values{
		"client_id":     {config.Kick.ClientID},
		"client_secret": {config.Kick.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	})
	if err != nil {
		logger.Error.Println("Kick token exchange failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Token exchange failed"))

		return
	}

	kickID, err := fetchJSONField(
		request.Context(), "https://id.kick.com/oauth/user-info",
		map[string]string{"Authorization": "Bearer " + tok.AccessToken},
		"sub",
	)
	if err != nil {
		logger.Error.Println("Kick user fetch failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Failed to fetch Kick user"))

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK || userID == 0 {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Internal Server Error"))

		return
	}

	scope := strings.Fields(tok.Scope)
	if err := postgres.UpsertOAuthToken(
		request.Context(), kickID, common.KICK,
		tok.AccessToken, tok.RefreshToken, scope, tok.ExpiresIn,
	); err != nil {
		logger.Warn.Println("Failed to upsert Kick OAuth token:", err)
	}

	sendHTML(writer, http.StatusOK, oauthSuccessHTML("kick"))
}

// ---------------------------------------------------------------------------
// AniList
// ---------------------------------------------------------------------------

func anilistAuthorizeHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := authUser(request)
	if !ok {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[string]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	config := utils.LoadConfig()
	state := newOAuthState(user.ID)
	redirectURI := strings.TrimRight(config.Anilist.OAuthURI, "/") + "/auth/anilist"

	params := url.Values{
		"client_id":     {config.Anilist.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"state":         {state},
	}

	target := fmt.Sprintf("%s?%s", anilistAuthorizeURL, params.Encode())
	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[string]{
		Data: &[]string{target},
	}, start)
}

func anilistCallbackHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	userID, ok := consumeOAuthState(state)
	if !ok {
		sendHTML(writer, http.StatusForbidden, oauthErrorHTML("Invalid or expired state"))

		return
	}

	config := utils.LoadConfig()
	redirectURI := strings.TrimRight(config.Anilist.OAuthURI, "/") + "/auth/anilist"

	tok, err := exchangeJSONToken(request.Context(), anilistTokenURL, map[string]any{
		"grant_type":    "authorization_code",
		"client_id":     config.Anilist.ClientID,
		"client_secret": config.Anilist.ClientSecret,
		"redirect_uri":  redirectURI,
		"code":          code,
	})
	if err != nil {
		logger.Error.Println("Anilist token exchange failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Token exchange failed"))

		return
	}

	anilistID, err := fetchAnilistUserID(request.Context(), tok.AccessToken)
	if err != nil {
		logger.Error.Println("Anilist user fetch failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Failed to fetch Anilist user"))

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK || userID == 0 {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Internal Server Error"))

		return
	}

	scope := strings.Fields(tok.Scope)
	if err := postgres.UpsertOAuthToken(
		request.Context(), anilistID, common.ANILIST,
		tok.AccessToken, tok.RefreshToken, scope, tok.ExpiresIn,
	); err != nil {
		logger.Warn.Println("Failed to upsert Anilist OAuth token:", err)
	}

	sendHTML(writer, http.StatusOK, oauthSuccessHTML("anilist"))
}

// ---------------------------------------------------------------------------
// Trakt
// ---------------------------------------------------------------------------

func traktAuthorizeHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := authUser(request)
	if !ok {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[string]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	config := utils.LoadConfig()
	state := newOAuthState(user.ID)
	redirectURI := strings.TrimRight(config.Trakt.OAuthURI, "/") + "/auth/trakt"

	params := url.Values{
		"client_id":     {config.Trakt.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"state":         {state},
	}

	target := fmt.Sprintf("%s?%s", traktAuthorizeURL, params.Encode())
	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[string]{
		Data: &[]string{target},
	}, start)
}

func traktCallbackHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	userID, ok := consumeOAuthState(state)
	if !ok {
		sendHTML(writer, http.StatusForbidden, oauthErrorHTML("Invalid or expired state"))

		return
	}

	config := utils.LoadConfig()
	redirectURI := strings.TrimRight(config.Trakt.OAuthURI, "/") + "/auth/trakt"

	tok, err := exchangeJSONToken(request.Context(), traktTokenURL, map[string]any{
		"code":          code,
		"client_id":     config.Trakt.ClientID,
		"client_secret": config.Trakt.ClientSecret,
		"redirect_uri":  redirectURI,
		"grant_type":    "authorization_code",
	})
	if err != nil {
		logger.Error.Println("Trakt token exchange failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Token exchange failed"))

		return
	}

	traktID, err := fetchJSONField(
		request.Context(), traktMeURL,
		map[string]string{
			"Authorization":     "Bearer " + tok.AccessToken,
			"trakt-api-version": "2",
			"trakt-api-key":     config.Trakt.ClientID,
		},
		"ids.slug",
	)
	if err != nil {
		logger.Error.Println("Trakt user fetch failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Failed to fetch Trakt user"))

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK || userID == 0 {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Internal Server Error"))

		return
	}

	scope := strings.Fields(tok.Scope)
	if err := postgres.UpsertOAuthToken(
		request.Context(), traktID, common.TRAKT,
		tok.AccessToken, tok.RefreshToken, scope, tok.ExpiresIn,
	); err != nil {
		logger.Warn.Println("Failed to upsert Trakt OAuth token:", err)
	}

	sendHTML(writer, http.StatusOK, oauthSuccessHTML("trakt"))
}

// ---------------------------------------------------------------------------
// FFZ — no JWT, no state, direct redirect
// ---------------------------------------------------------------------------

func ffzAuthorizeHandler(writer http.ResponseWriter, request *http.Request) {
	config := utils.LoadConfig()
	redirectURI := strings.TrimRight(config.FFZ.OAuthURI, "/") + "/auth/ffz"

	params := url.Values{
		"client_id":     {config.FFZ.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {ffzScopes},
	}

	http.Redirect(writer, request, fmt.Sprintf("%s?%s", ffzAuthorizeURL, params.Encode()), http.StatusFound)
}

func ffzCallbackHandler(writer http.ResponseWriter, request *http.Request) {
	code := request.URL.Query().Get("code")

	config := utils.LoadConfig()
	redirectURI := strings.TrimRight(config.FFZ.OAuthURI, "/") + "/auth/ffz"

	tok, err := exchangeFormToken(request.Context(), ffzTokenURL, url.Values{
		"client_id":     {config.FFZ.ClientID},
		"client_secret": {config.FFZ.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		logger.Error.Println("FFZ token exchange failed:", err)
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("FFZ token exchange failed"))

		return
	}

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Internal Server Error"))

		return
	}

	scope := strings.Fields(tok.Scope)
	if err := postgres.UpsertOAuthToken(
		request.Context(), config.FFZ.ID, common.FFZ,
		tok.AccessToken, tok.RefreshToken, scope, tok.ExpiresIn,
	); err != nil {
		logger.Warn.Println("Failed to upsert FFZ OAuth token:", err)
	}

	sendHTML(writer, http.StatusOK, oauthSuccessHTML("ffz"))
}

// ---------------------------------------------------------------------------
// Steam — OpenID 2.0
// ---------------------------------------------------------------------------

func steamAuthorizeHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	user, ok := authUser(request)
	if !ok {
		api.GenericResponse(writer, http.StatusUnauthorized, common.GenericResponse[string]{
			Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
		}, start)

		return
	}

	// Derive the base URL from the incoming request to ensure Steam OpenID
	// callbacks use the correct host and scheme for this API instance.
	scheme := "https"
	if forwarded := request.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		// Use the first value if multiple are provided.
		scheme = strings.Split(forwarded, ",")[0]
	} else if request.TLS == nil {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s/", scheme, request.Host)
	returnTo := fmt.Sprintf("%sauth/steam?user_id=%d", baseURL, user.ID)
	realm := baseURL

	params := url.Values{
		"openid.ns":         {steamOpenIDNS},
		"openid.mode":       {"checkid_setup"},
		"openid.return_to":  {returnTo},
		"openid.realm":      {realm},
		"openid.identity":   {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.claimed_id": {"http://specs.openid.net/auth/2.0/identifier_select"},
	}

	target := fmt.Sprintf("%s?%s", steamOpenIDURL, params.Encode())
	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[string]{
		Data: &[]string{target},
	}, start)
}

func steamCallbackHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()

	// Re-verify the OpenID assertion with Steam
	verifyParams := url.Values{}
	maps.Copy(verifyParams, query)
	verifyParams.Set("openid.mode", "check_authentication")

	ctx := request.Context()
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		steamOpenIDURL,
		strings.NewReader(verifyParams.Encode()),
	)
	if err != nil {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Steam verification failed"))

		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req) //nolint:gosec
	if err != nil || resp.StatusCode != http.StatusOK {
		sendHTML(writer, http.StatusForbidden, oauthErrorHTML("Steam verification failed"))

		return
	}
	defer resp.Body.Close() //nolint:errcheck

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "is_valid:true") {
		sendHTML(writer, http.StatusForbidden, oauthErrorHTML("Steam assertion invalid"))

		return
	}

	claimedID := query.Get("openid.claimed_id")
	// claimed_id ends with the 64-bit SteamID, e.g. https://steamcommunity.com/openid/id/76561198...
	parts := strings.Split(claimedID, "/")
	steamID64Str := parts[len(parts)-1]

	var steamID64 int64
	if _, err := fmt.Sscanf(steamID64Str, "%d", &steamID64); err != nil {
		sendHTML(writer, http.StatusBadRequest, oauthErrorHTML("Invalid Steam ID"))

		return
	}
	steamFriendCode := fmt.Sprintf("%d", steamID64-steamFriendOffset)

	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK {
		sendHTML(writer, http.StatusInternalServerError, oauthErrorHTML("Internal Server Error"))

		return
	}

	if err := postgres.UpsertOAuthToken(
		request.Context(), steamID64Str, common.STEAM,
		"", "", []string{}, 0,
	); err != nil {
		logger.Warn.Println("Failed to upsert Steam token:", err)
	}

	sendHTML(writer, http.StatusOK, oauthPostMessage(map[string]any{
		"platform":    "steam",
		"ok":          true,
		"steam_id":    steamID64Str,
		"friend_code": steamFriendCode,
	}))
}

// ---------------------------------------------------------------------------
// OAuth helpers
// ---------------------------------------------------------------------------

type simpleTokenResponse struct { //nolint:govet
	AccessToken  string `json:"access_token"`  //nolint:gosec
	RefreshToken string `json:"refresh_token"` //nolint:gosec
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type exchangeOption func(*http.Request)

func withBasicAuth(clientID, clientSecret string) exchangeOption {
	return func(req *http.Request) {
		req.SetBasicAuth(clientID, clientSecret)
	}
}

// exchangeFormToken does a standard application/x-www-form-urlencoded code exchange.
func exchangeFormToken(
	ctx context.Context,
	tokenURL string,
	params url.Values,
	opts ...exchangeOption,
) (*simpleTokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for _, opt := range opts {
		opt(req)
	}

	client := &http.Client{Timeout: oauthHTTPTimeout}

	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 { //nolint:mnd
		b, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("token exchange returned %d: %s", resp.StatusCode, string(b)) //nolint:err113
	}

	var tok simpleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}

	return &tok, nil
}

// exchangeJSONToken does a code exchange with application/json body.
func exchangeJSONToken(ctx context.Context, tokenURL string, body map[string]any) (*simpleTokenResponse, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: oauthHTTPTimeout}

	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 { //nolint:mnd
		b, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("token exchange returned %d: %s", resp.StatusCode, string(b)) //nolint:err113
	}

	var tok simpleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}

	return &tok, nil
}

// fetchJSONField makes an authenticated request and returns a top-level string field.
// Nested fields can be accessed with dot notation (e.g. "ids.slug").
func fetchJSONField(
	ctx context.Context,
	endpoint string,
	headers map[string]string,
	field string,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: oauthHTTPTimeout}

	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	parts := strings.SplitN(field, ".", 2) //nolint:mnd
	val, exists := data[parts[0]]

	if !exists {
		return "", fmt.Errorf("field %q not found in response", parts[0]) //nolint:err113
	}

	if len(parts) == 2 { //nolint:mnd
		nested, ok := val.(map[string]any)
		if !ok {
			return "", fmt.Errorf("field %q is not an object", parts[0]) //nolint:err113
		}

		val, exists = nested[parts[1]]
		if !exists {
			return "", fmt.Errorf("field %q not found in nested object", parts[1]) //nolint:err113
		}
	}

	return fmt.Sprintf("%v", val), nil
}

// fetchDiscordUserID calls /users/@me and returns the Discord user ID.
func fetchDiscordUserID(ctx context.Context, accessToken string) (string, error) {
	return fetchJSONField(
		ctx, discordMeURL,
		map[string]string{"Authorization": "Bearer " + accessToken},
		"id",
	)
}

// fetchAnilistUserID queries the AniList GraphQL API for the authenticated user's ID.
func fetchAnilistUserID(ctx context.Context, accessToken string) (string, error) {
	query := `{"query":"{Viewer{id}}"}`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anilistGQLURL, strings.NewReader(query))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: oauthHTTPTimeout}

	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Data struct {
			Viewer struct {
				ID int `json:"id"`
			} `json:"Viewer"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", result.Data.Viewer.ID), nil
}
