## ADDED Requirements

### Requirement: Config struct with go-redis v9 options
The `redis/v2` package SHALL provide a `Config` struct with the following fields: `Addrs []string`, `Username string`, `Password string`, `DB int`, `Mode Mode` (Standalone or Cluster), `PoolSize int`, `MinIdleConns int`, `MaxConnAge time.Duration`, `IdleTimeout time.Duration`, `ConnMaxIdleTime time.Duration`, `EnableMetrics bool`, `MetricInterval time.Duration`. The `ConnMaxIdleTime` field SHALL default to `60s` via the `default` struct tag.

#### Scenario: Config with ConnMaxIdleTime default
- **WHEN** a `Config` is created with `ConnMaxIdleTime` left as zero value
- **THEN** the underlying v9 `redis.Options`/`redis.ClusterOptions` SHALL set `ConnMaxIdleTime` to `60s`

#### Scenario: Config with explicit ConnMaxIdleTime
- **WHEN** a `Config` is created with `ConnMaxIdleTime: 120 * time.Second`
- **THEN** the underlying v9 options SHALL set `ConnMaxIdleTime` to `120s`

#### Scenario: Config backward compatibility
- **WHEN** a caller migrates from `redis/v1.Config` to `redis/v2.Config` without setting `ConnMaxIdleTime`
- **THEN** all existing fields (`PoolSize`, `MinIdleConns`, `MaxConnAge`, `IdleTimeout`, `EnableMetrics`, `MetricInterval`) SHALL behave identically to v1

### Requirement: NewRedisClient factory function
The package SHALL export `NewRedisClient(ctx context.Context, config *Config) (RedisClient, error)` that creates either a standalone `redis.Client` or cluster `redis.ClusterClient` based on `config.Mode`, sets connection pool parameters from Config, maps `MaxConnAge` to v9's `ConnMaxLifetime`, pings the server, and returns a `RedisClient` implementation.

#### Scenario: Standalone mode
- **WHEN** `NewRedisClient` is called with `Mode: StandaloneMode` and valid `Addrs`
- **THEN** a `redis.Client` backed by go-redis v9 SHALL be created, Ping SHALL succeed, and the returned `RedisClient` SHALL route all commands through that client

#### Scenario: Cluster mode
- **WHEN** `NewRedisClient` is called with `Mode: ClusterMode` and multiple `Addrs`
- **THEN** a `redis.ClusterClient` backed by go-redis v9 SHALL be created, Ping SHALL succeed, and the returned `RedisClient` SHALL route all commands through the cluster client

#### Scenario: Connection failure
- **WHEN** `NewRedisClient` is called with unreachable `Addrs`
- **THEN** the Ping step SHALL return an error and `NewRedisClient` SHALL return that error

### Requirement: RedisClient interface
The package SHALL export a `RedisClient` interface with the same method signatures as v1: `Ping`, `GetString`, `SetString`, `Del`, `Exists`, `GetHashAll`, `SetHash`, `IncrBy`, `GetList`, `PushList`, `PopList`, `AddSet`, `RemoveSet`, `GetSetMembers`, `IsSetMember`, `ZAdd`, `ZRem`, `ZRangeByScore`, `ZScore`, `Close`. All methods SHALL accept `context.Context` as the first parameter.

#### Scenario: All Redis operations work through the interface
- **WHEN** a caller uses any method on the `RedisClient` interface
- **THEN** the command SHALL be executed via go-redis v9 and return the correct result

#### Scenario: Key not found
- **WHEN** `GetString` is called for a key that does not exist
- **THEN** the method SHALL return `("", nil)` (matching v1 behavior where `redis.Nil` is swallowed)

### Requirement: Encoding helpers
The package SHALL export `JSONSet(ctx, key, value, expiration)` and `JSONGet(ctx, key, dest)` methods that serialize/deserialize complex Go types (slices, maps, structs) as JSON strings in Redis. `JSONSet` SHALL use `json.Marshal` and `Set`. `JSONGet` SHALL use `Get` and `json.Unmarshal`.

#### Scenario: JSONSet with struct value
- **WHEN** `JSONSet` is called with a struct value
- **THEN** the value SHALL be marshaled to JSON and stored in Redis as a string

#### Scenario: JSONGet with slice destination
- **WHEN** `JSONGet` is called with a pointer to a slice
- **THEN** the JSON string from Redis SHALL be unmarshaled into the slice

#### Scenario: JSONGet key not found
- **WHEN** `JSONGet` is called for a non-existent key
- **THEN** the method SHALL return `nil` error without modifying the destination (matching v1 behavior)

### Requirement: Mode constants
The package SHALL export `Mode` type with constants `StandaloneMode Mode = "standalone"` and `ClusterMode Mode = "cluster"`.

#### Scenario: Mode constant usage
- **WHEN** a caller sets `config.Mode = ClusterMode`
- **THEN** `NewRedisClient` SHALL create a cluster client
