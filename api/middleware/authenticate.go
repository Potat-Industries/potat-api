package middleware

import (
	"net/http"
)

func AuthenticateRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// DO SOMETHING I GUESS

		next(w, r)
	}
}