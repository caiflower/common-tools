package dbv1

import "time"

type Config struct {
	Dialect               string        `yaml:"dialect" default:"mysql" json:"dialect"`
	Url                   string        `yaml:"url" default:"127.0.0.1:3306" json:"url"`
	DbName                string        `yaml:"dbName" json:"dbName"`
	User                  string        `yaml:"user" json:"user"`
	Password              string        `yaml:"password" json:"password"`
	EnablePasswordEncrypt bool          `yaml:"enablePasswordEncrypt" json:"enablePasswordEncrypt"`
	Charset               string        `yaml:"charset" default:"utf8mb4" json:"charset"`
	MaxOpen               int           `yaml:"maxOpen" default:"200" json:"maxOpen"`
	MaxIdle               int           `yaml:"maxIdle" default:"20" json:"maxIdle"`
	ConnMaxLifetime       int           `yaml:"connMaxLifetime" default:"28800" json:"connMaxLifetime"`
	Plural                bool          `yaml:"plural" json:"plural"`
	Debug                 bool          `yaml:"debug" json:"debug"`
	EnableMetric          bool          `yaml:"enableMetric" json:"enableMetric"`
	TransactionTimeout    time.Duration `yaml:"transactionTimeout" json:"transactionTimeout" default:"30s"`
}
