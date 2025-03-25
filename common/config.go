// Package common provides common types and configurations used throughout the application.
package common

// Config holds the configuration for the application, including database and service settings.
type Config struct {
	Postgres   SQLConfig    `json:"postgres"`
	Clickhouse SQLConfig    `json:"clickhouse"`
	Twitch     TwitchConfig `json:"twitch"`
	Redis      RedisConfig  `json:"redis"`
	API        APIConfig    `json:"api"`
	Socket     APIConfig    `json:"socket"`
	Redirects  APIConfig    `json:"redirects"`
	Uploader   APIConfig    `json:"uploader"`
	Prometheus APIConfig    `json:"prometheus"`
	Haste      HasteConfig  `json:"haste"`
	Nats       BoolConfig   `json:"nats"`
	Loops      BoolConfig   `json:"loops"`
}

// TwitchConfig holds the configuration for Twitch API integration.
type TwitchConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	OauthURI     string `json:"oauth_uri"`
}

// BoolConfig holds the configuration for the simple enablable service.
type BoolConfig struct {
	Enabled bool `json:"enabled"`
}

// APIConfig holds the configuration for various API services, including host, port, and authentication settings.
type APIConfig struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	AuthKey string `json:"authkey,omitempty"`
	Enabled bool   `json:"enabled"`
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
	Password string `json:"password"`
	Database string `json:"database"`
}

// RedisConfig holds the configuration for Redis, including host and port.
type RedisConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}
