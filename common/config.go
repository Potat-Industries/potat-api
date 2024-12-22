package common

type Config struct {
	API        APIConfig			`json:"api"`
	Redirects  APIConfig			`json:"redirects"`
	Uploader   APIConfig			`json:"uploader"`
	RabbitMQ   RabbitMQConfig	`json:"rabbitmq"`
	Haste			 HasteConfig		`json:"haste"`
	Prometheus APIConfig			`json:"prometheus"`
	Postgres   SQLConfig   		`json:"postgres"`
	Clickhouse SQLConfig 			`json:"clickhouse"`
	Redis      RedisConfig    `json:"redis"`
}

type APIConfig struct {
	Enabled bool 		`json:"enabled"`
	Host string  		`json:"host"`
	Port string  		`json:"port"`
	AuthKey string 	`json:"authkey,omitempty"`
}

type RabbitMQConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type HasteConfig struct {
	Enabled 	bool 	 `json:"enabled"`
	Host 			string `json:"host"`
	Port		  string `json:"port"`
	KeyLength int		 `json:"keyLength"`
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
