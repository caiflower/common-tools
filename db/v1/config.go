package v1

type Config struct {
	Dialect         string `yaml:"dialect" default:"mysql"`
	Url             string `yaml:"url" default:"127.0.0.1:3306"`
	DbName          string `yaml:"dbName"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Charset         string `yaml:"charset" default:"utf8mb4"`
	MaxOpen         int    `yaml:"maxOpen" default:"200"`
	MaxIdle         int    `yaml:"maxIdle" default:"20"`
	ConnMaxLifetime int    `yaml:"connMaxLifetime" default:"28800"`
	Plural          bool   `yaml:"plural"`
	Debug           bool   `yaml:"debug"`
	EnableMetric    bool   `yaml:"enable_metric"`
}
