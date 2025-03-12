package common

var UserLevels = struct {
	Blacklisted int
	Admin       int
	Mod         int
	User        int
	Developer   int
}{
	Blacklisted: 0,
	Admin:       1,
	Mod:         2,
	User:        3,
	Developer:   4,
}

type Config struct {
	Postgres   SQLConfig      `json:"postgres"`
	Clickhouse SQLConfig      `json:"clickhouse"`
	Twitch     TwitchConfig   `json:"twitch"`
	Redis      RedisConfig    `json:"redis"`
	RabbitMQ   RabbitMQConfig `json:"rabbitmq"`
	API        APIConfig      `json:"api"`
	Socket     APIConfig      `json:"socket"`
	Redirects  APIConfig      `json:"redirects"`
	Uploader   APIConfig      `json:"uploader"`
	Prometheus APIConfig      `json:"prometheus"`
	Haste      HasteConfig    `json:"haste"`
	Loops      LoopsConfig    `json:"loops"`
}

type TwitchConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	OauthURI     string `json:"oauth_uri"`
}

type LoopsConfig struct {
	Enabled bool `json:"enabled"`
}

type APIConfig struct {
	Host    string `json:"host"`
	Port    string `json:"port"`
	AuthKey string `json:"authkey,omitempty"`
	Enabled bool   `json:"enabled"`
}

type RabbitMQConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Enabled  bool   `json:"enabled"`
}

type HasteConfig struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	KeyLength int    `json:"keyLength"`
	Enabled   bool   `json:"enabled"`
}

type SQLConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type RedisConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}
