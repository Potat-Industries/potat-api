package utils

import (
	"fmt"
	"time"
	"bytes"
	"strings"
	"net/http"
	"encoding/json"
)

type GqlQuery struct {
	Query string `json:"query"`
}

type StvResponse struct {
	Data   map[string]StvUser `json:"data"`
	Errors []StvError      `json:"errors"`
}

type StvUser struct {
	ID        string `json:"id"`
	AvatarURL string `json:"avatar_url"`
}

type StvError struct {
	Message   string      `json:"message"`
	Locations []Location  `json:"locations"`
}

type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

var Make = &http.Client{
	Timeout: time.Second * 10,
}

func MakeRequest(
	method string,
	url string,
	headers map[string]string,
	body *bytes.Buffer,
) (*http.Response, error) {
	if body == nil {
		body = &bytes.Buffer{}
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := Make.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func BatchLoadStvData(ids []string) ([]StvUser, error) {
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
	err := json.NewEncoder(&body).Encode(GqlQuery{Query: query})
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	res, err := MakeRequest(
		"POST",
		"https://7tv.io/v3/gql",
		headers,
		&body,
	)

	if err != nil {
		return nil, err
	}

	Warn.Printf("Response: %v", res)

	var response StvResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("error in 7TV response: %v", response.Errors)
	}

	var users []StvUser
	for _, user := range response.Data {
		users = append(users, user)
	}

	return users, nil
}

func ValidateHelixToken(token string) (bool, error) {
	res, err := MakeRequest(
		"GET",
		"https://id.twitch.tv/oauth2/validate",
		map[string]string{"Authorization": "OAuth " + token},
		nil,
	)

	if err != nil {
		return false, err
	}

	defer res.Body.Close()

	return res.StatusCode != 401, nil
}



