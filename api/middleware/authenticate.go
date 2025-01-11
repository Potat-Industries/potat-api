package middleware

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"potat-api/common"
	"potat-api/common/db"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthenticatedUser string

type PotatClaims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

var JWTSecret []byte

const AuthedUser = AuthenticatedUser("authenticated-user")

func SetJWTSecret(s string) {
	JWTSecret = []byte(s)
}

func SetStaticAuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func (next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
			if !verifySimpleAuthKey(auth, secret) {
				http.Error(w, http.StatusText(http.StatusTeapot), http.StatusTeapot)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func verifySimpleAuthKey(provided, possessed string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(possessed)) == 1
}

func SetDynamicAuthMiddleware() func(http.Handler) http.Handler {
	return func (next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if token == "" {
				http.Error(w, http.StatusText(http.StatusTeapot), http.StatusTeapot)
				return
			}

			ok, user := verifyDynamicAuth(token, r.Context())
			if !ok {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), AuthedUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func verifyDynamicAuth(token string, ctx context.Context) (bool, *common.User) {
	claims, err := verifyJWT(token)
	if err != nil {
		return false, &common.User{}
	}

	user, err := db.Postgres.GetUserByInternalID(ctx, claims.UserID)
	if err != nil {
		return false, &common.User{}
	}

	return true, user
}

func jwtKeyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	return JWTSecret, nil
}

func verifyJWT(tokenString string) (*PotatClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PotatClaims{}, jwtKeyFunc)
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*PotatClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func CreateJWT(userID int) (string, error) {
	claims := PotatClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(183 * 24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(JWTSecret)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}


