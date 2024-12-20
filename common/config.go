package common

type Config struct {
	API        APIConfig				`json:"api"`
	Redirects  RedirectsConfig	`json:"redirects"`
	RabbitMQ   RabbitMQConfig		`json:"rabbitmq"`
	Haste			 HasteConfig			`json:"haste"`
	Prometheus PrometheusConfig	`json:"prometheus"`
	Postgres   PostgresConfig   `json:"postgres"`
	Clickhouse ClickhouseConfig `json:"clickhouse"`
	Redis      RedisConfig      `json:"redis"`
}

type APIConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type RabbitMQConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type RedirectsConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type HasteConfig struct {
	Host string 		 `json:"host"`
	Port string 		 `json:"port"`
	KeyLength int		 `json:"keyLength"`
}

type PrometheusConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type ClickhouseConfig struct {
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