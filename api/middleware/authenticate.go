package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func SetAuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func (next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
			if !verifyAuthKey(auth, secret) {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func verifyAuthKey(provided, possessed string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(possessed)) == 1
}
