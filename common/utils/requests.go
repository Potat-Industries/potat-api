package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
)

var (
	errEmptyToken  = fmt.Errorf("empty token")
	errFailRefresh = fmt.Errorf("failed to refresh token")
	errStvFail     = fmt.Errorf("7tv request failed")
)

type gqlQuery struct {
	Query string `json:"query"`
}

type stvResponse struct {
	Data   map[string]StvUser `json:"data"`
	Errors []stvError         `json:"errors"`
}

// StvUser represents a user from 7TV with their ID and avatar URL.
type StvUser struct {
	ID        string `json:"id"`
	AvatarURL string `json:"avatar_url"`
}

type stvError struct {
	Message   string     `json:"message"`
	Locations []location `json:"locations"`
}

type location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func makeRequest(
	ctx context.Context,
	method string,
	url string,
	headers map[string]string,
	body *bytes.Buffer,
) (*http.Response, error) {
	if body == nil {
		body = &bytes.Buffer{}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// BatchLoadStvData fetches user data from 7TV for a batch of Twitch user IDs.
func BatchLoadStvData(ctx context.Context, ids []string) ([]StvUser, error) {
	var queryParts []string

	for _, id := range ids {
		queryParts = append(queryParts, fmt.Sprintf(
			`u%s:userByConnection(platform:TWITCH id:"%s"){id avatar_url}`,
			id,
			id,
		))
	}

	query := fmt.Sprintf(`{%s}`, strings.Join(queryParts, " "))

	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(gqlQuery{Query: query})
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	res, err := makeRequest(
		ctx,
		"POST",
		"https://7tv.io/v3/gql",
		headers,
		&body,
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err = res.Body.Close(); err != nil {
			logger.Error.Println("Error closing response body:", err)
		}
	}()

	logger.Warn.Printf("Response: %v", res)

	var response stvResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		return nil, errStvFail
	}

	var users []StvUser
	for _, user := range response.Data {
		users = append(users, user)
	}

	return users, nil
}

// ValidateHelixToken checks if a given Twitch OAuth token is valid.
func ValidateHelixToken(
	ctx context.Context,
	token string,
	returnAll bool,
) (bool, *common.TwitchValidation, error) {
	if token == "" {
		return false, &common.TwitchValidation{}, errEmptyToken
	}

	res, err := makeRequest(
		ctx,
		"GET",
		"https://id.twitch.tv/oauth2/validate",
		map[string]string{"Authorization": "OAuth " + token},
		nil,
	)
	if err != nil {
		return false, &common.TwitchValidation{}, err
	}

	defer func() {
		if err = res.Body.Close(); err != nil {
			logger.Error.Println("Error closing response body:", err)
		}
	}()

	if !returnAll {
		return res.StatusCode != 401, &common.TwitchValidation{}, nil
	}

	var validation common.TwitchValidation
	err = json.NewDecoder(res.Body).Decode(&validation)
	if err != nil {
		return false, &common.TwitchValidation{}, err
	}

	return res.StatusCode != 401, &validation, nil
}

// RefreshHelixToken refreshes a Twitch OAuth token using the refresh token flow.
func RefreshHelixToken(
	ctx context.Context,
	config common.Config,
	token string,
) (*common.GenericOAUTHResponse, error) {
	if token == "" {
		return nil, errEmptyToken
	}

	if config.Twitch.ClientID == "" || config.Twitch.ClientSecret == "" {
		logger.Debug.Fatalln("Twitch client ID or secret not set")
	}

	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {token},
		"client_id":     {config.Twitch.ClientID},
		"client_secret": {config.Twitch.ClientSecret},
	}.Encode()

	res, err := makeRequest(
		ctx,
		"POST",
		"https://id.twitch.tv/oauth2/token",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		bytes.NewBuffer([]byte(params)),
	)

	if res.StatusCode != 200 {
		return nil, errFailRefresh
	}

	if err != nil {
		return nil, err
	}

	defer func() {
		if err = res.Body.Close(); err != nil {
			logger.Error.Println("Error closing response body:", err)
		}
	}()

	var response common.GenericOAUTHResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
