package get

import (
	"net/http"

	"github.com/Potat-Industries/potat-api/api"
)

const docs = "https://potat.app/api/docs"

func Docs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, docs, http.StatusMovedPermanently)
}

func init() {
	api.SetRoute(api.Route{
		Path:    "/docs",
		Method:  http.MethodGet,
		Handler: Docs,
		UseAuth: false,
	})
}
