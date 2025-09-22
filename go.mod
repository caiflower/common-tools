module github.com/caiflower/common-tools

go 1.16

replace github.com/json-iterator/go v1.1.12 => github.com/caiflower/json-iterator v1.1.12

require (
	github.com/Shopify/sarama v1.34.1
	github.com/andybalholm/brotli v1.1.1
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.0.2
	github.com/eapache/go-resiliency v1.3.0 // indirect
	github.com/go-redis/redis/extra/rediscmd/v8 v8.11.4
	github.com/go-redis/redis/v8 v8.11.4
	github.com/go-sql-driver/mysql v1.7.1
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/json-iterator/go v1.1.12
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/modern-go/gls v0.0.0-20220109145502-612d0167dce5
	github.com/modern-go/reflect2 v1.0.2
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/prometheus/client_golang v1.12.2
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.7.5 // indirect
	github.com/uptrace/bun v1.0.19
	github.com/uptrace/bun/dialect/mysqldialect v1.0.19
	github.com/uptrace/uptrace-go v1.3.1
	github.com/xdg-go/scram v1.1.1
	github.com/xuri/excelize/v2 v2.6.1
	go.opentelemetry.io/otel v1.7.0
	go.opentelemetry.io/otel/metric v0.26.0
	go.opentelemetry.io/otel/trace v1.7.0
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	gopkg.in/yaml.v2 v2.4.0
)
