package helix

import (
	"bytes"
	"net/http"
	"time"
)

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

func ValidateToken(token string) (bool, error) {
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
