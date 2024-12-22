module github.com/caiflower/common-tools

go 1.16

replace github.com/json-iterator/go v1.1.12 => github.com/caiflower/json-iterator v1.1.12

require (
	github.com/andybalholm/brotli v1.1.1
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.0.2
	github.com/go-redis/redis/v8 v8.11.4
	github.com/go-sql-driver/mysql v1.7.1
	github.com/google/uuid v1.6.0
	github.com/json-iterator/go v1.1.12
	github.com/modern-go/gls v0.0.0-20220109145502-612d0167dce5
	github.com/modern-go/reflect2 v1.0.2
	github.com/prometheus/client_golang v1.12.2
	github.com/robfig/cron/v3 v3.0.1
	github.com/uptrace/bun v1.0.19
	github.com/uptrace/bun/dialect/mysqldialect v1.0.19
	gopkg.in/yaml.v2 v2.4.0
)
