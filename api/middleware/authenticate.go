// Package middleware provides middleware for authenticating, logging, and rate limiting requests.
package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/logger"
)

var (
	errUnexpectedSign = errors.New("unexpected signing method")
	errInvalidToken   = errors.New("invalid token")
)

type unauthorizedResponse = common.GenericResponse[string]

// AuthenticatedUser is the key for the authenticated user in the request context.
type AuthenticatedUser string

type potatClaims struct {
	jwt.RegisteredClaims
	UserID int `json:"user_id"`
}

type unauthFunc func(
	w http.ResponseWriter,
	status int,
	response interface{},
	start time.Time,
)

// Authenticator provides middleware for authenticating requests.
type Authenticator struct {
	unauthorizedFunc unauthFunc
	secret           []byte
}

// AuthedUser is the key for the authenticated user in the request context.
const AuthedUser = AuthenticatedUser("authenticated-user")

// NewAuthenticator creates a new authenticator with the provided secret.
func NewAuthenticator(secret string, unauthorizedFunc unauthFunc) *Authenticator {
	return &Authenticator{
		secret:           []byte(secret),
		unauthorizedFunc: unauthorizedFunc,
	}
}

func (a *Authenticator) sendUnauthorized(w http.ResponseWriter) {
	logger.Warn.Println("Unauthorized request")
	a.unauthorizedFunc(w, http.StatusTeapot, unauthorizedResponse{
		Data:   &[]string{},
		Errors: &[]common.ErrorMessage{{Message: "Unauthorized"}},
	}, time.Now())
}

// SetStaticAuthMiddleware returns a middleware that verifies the provided static auth key.
func (a *Authenticator) SetStaticAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
			if !a.verifySimpleAuthKey(auth) {
				a.sendUnauthorized(w)

				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (a *Authenticator) verifySimpleAuthKey(provided string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), a.secret) == 1
}

// SetDynamicAuthMiddleware returns a middleware that verifies the provided dynamic auth token.
func (a *Authenticator) SetDynamicAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			token := request.Header.Get("Authorization")
			if token == "" {
				a.sendUnauthorized(writer)

				return
			}

			ok, user := a.verifyDynamicAuth(request.Context(), token)
			if !ok {
				a.sendUnauthorized(writer)

				return
			}

			ctx := context.WithValue(request.Context(), AuthedUser, user)
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

func (a *Authenticator) verifyDynamicAuth(ctx context.Context, token string) (bool, *common.User) {
	token = strings.Replace(token, "Bearer ", "", 1)
	claims, err := a.verifyJWT(token)
	if err != nil {
		return false, &common.User{}
	}

	postgres, ok := ctx.Value(PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")

		return false, &common.User{}
	}

	user, err := postgres.GetUserByInternalID(ctx, claims.UserID)
	if err != nil {
		logger.Warn.Println("Error fetching authenticated user: ", err)

		return false, &common.User{}
	}

	return true, user
}

func (a *Authenticator) jwtKeyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("%w: unexpected signing method: %v", errUnexpectedSign, token.Header["alg"])
	}

	return a.secret, nil
}

func (a *Authenticator) verifyJWT(tokenString string) (*potatClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &potatClaims{}, a.jwtKeyFunc)
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*potatClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errInvalidToken
}

// CreateJWT creates a new JWT token for the provided user ID.
func (a *Authenticator) CreateJWT(userID int) (string, error) {
	claims := potatClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(183 * 24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(a.secret)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
