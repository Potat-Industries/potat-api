// Package common provides common types and configurations used throughout the application.
package common

// Config holds the configuration for the application, including database and service settings.
type Config struct {
	Postgres   SQLConfig     `json:"postgres"`
	Clickhouse SQLConfig     `json:"clickhouse"`
	Twitch     TwitchConfig  `json:"twitch"`
	Discord    DiscordConfig `json:"discord"`
	Spotify    SpotifyConfig `json:"spotify"`
	Kick       KickConfig    `json:"kick"`
	Anilist    AnilistConfig `json:"anilist"`
	Trakt      TraktConfig   `json:"trakt"`
	FFZ        FFZConfig     `json:"ffz"`
	STV        STVConfig     `json:"stv"`
	BTTV       BTTVConfig    `json:"bttv"`
	Misc       MiscConfig    `json:"misc"`
	Redis      RedisConfig   `json:"redis"`
	API        APIConfig     `json:"api"`
	Socket     APIConfig     `json:"socket"`
	Redirects  APIConfig     `json:"redirects"`
	Uploader   APIConfig     `json:"uploader"`
	Prometheus APIConfig     `json:"prometheus"`
	Haste      HasteConfig   `json:"haste"`
	Nats       BoolConfig    `json:"nats"`
	Loops      BoolConfig    `json:"loops"`
}

// TwitchConfig holds the configuration for Twitch API integration.
type TwitchConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` //nolint:gosec
	OauthURI     string `json:"oauth_uri"`
}

// DiscordConfig holds the configuration for Discord OAuth and bot integration.
type DiscordConfig struct {
	ID           string `json:"id"`
	OAuth        string `json:"oauth"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` //nolint:gosec
	OAuthURI     string `json:"oauth_uri"`
}

// SpotifyConfig holds the configuration for Spotify OAuth integration.
type SpotifyConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` //nolint:gosec
	OAuthURI     string `json:"oauth_uri"`
}

// KickConfig holds the configuration for Kick OAuth integration (PKCE flow).
type KickConfig struct {
	ID           string `json:"id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` //nolint:gosec
	OAuthURI     string `json:"oauth_uri"`
	OAuth        string `json:"oauth"`
}

// AnilistConfig holds the configuration for AniList OAuth integration.
type AnilistConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` //nolint:gosec
	OAuthURI     string `json:"oauth_uri"`
}

// TraktConfig holds the configuration for Trakt OAuth integration.
type TraktConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` //nolint:gosec
	OAuthURI     string `json:"oauth_uri"`
}

// FFZConfig holds the configuration for FrankerFaceZ OAuth integration.
type FFZConfig struct {
	ID           string `json:"id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` //nolint:gosec
	Token        string `json:"token"`
	Refresh      string `json:"refresh"`
	OAuthURI     string `json:"oauth_uri"`
}

// STVConfig holds the configuration for 7TV bot token.
type STVConfig struct {
	Token string `json:"token"`
	ID    string `json:"id"`
}

// BTTVConfig holds the configuration for BTTV bot token.
type BTTVConfig struct {
	Token string `json:"token"`
	ID    string `json:"id"`
}

// SteamConfig holds the Steam Web API key.
type SteamConfig struct {
	Key string `json:"key"`
}

// MiscConfig holds miscellaneous third-party service configurations.
type MiscConfig struct {
	Steam SteamConfig `json:"steam"`
}

// BoolConfig holds the configuration for the simple enablable service.
type BoolConfig struct {
	Enabled bool `json:"enabled"`
}

type APIConfig struct { //nolint:govet
	CORSOrigins  []string `json:"cors_origins,omitempty"`
	CookieDomain string   `json:"cookie_domain,omitempty"`
	Host         string   `json:"host"`
	Port         string   `json:"port"`
	AuthKey      string   `json:"authkey,omitempty"` //nolint:gosec
	Enabled      bool     `json:"enabled"`
}

// HasteConfig holds the configuration for the Hastebin service, including host, port, key length,
// and whether it is enabled.
type HasteConfig struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	KeyLength int    `json:"keyLength"`
	Enabled   bool   `json:"enabled"`
}

// SQLConfig holds the configuration for SQL databases, including host, port, user, password, and database name.
type SQLConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"` //nolint:gosec
	Database string `json:"database"`
}

// RedisConfig holds the configuration for Redis, including host and port.
type RedisConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}
