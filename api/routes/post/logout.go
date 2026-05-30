// Package post contains routes for http.MethodPost requests.
package post

import (
	"net/http"
	"time"

	"github.com/Potat-Industries/potat-api/api"
	"github.com/Potat-Industries/potat-api/common/utils"
)

func init() {
	api.SetRoute(api.Route{
		Path:    "/logout",
		Method:  http.MethodPost,
		Handler: logoutHandler,
		UseAuth: false,
	})
}

func logoutHandler(writer http.ResponseWriter, request *http.Request) {
	config := utils.LoadConfig()

	http.SetCookie(writer, &http.Cookie{
		Name:     "authorization",
		Value:    "",
		Path:     "/",
		Domain:   config.API.CookieDomain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	writer.WriteHeader(http.StatusNoContent)
}
