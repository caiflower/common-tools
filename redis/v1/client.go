/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package redisv1

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	xredis "github.com/caiflower/common-tools/redis"
	"github.com/go-redis/redis/v8"
)

var (
	ErrNil = errors.New("redis key does not exist")
)

type RedisClient interface {
	// Client management
	GetRedis() redis.Cmdable
	AddHook(hook redis.Hook)
	GetKey(k string) string

	// String
	Set(ctx context.Context, k string, v interface{}) error
	SetPeriod(ctx context.Context, k string, v interface{}, period time.Duration) error
	SetNX(ctx context.Context, k string, v interface{}) (bool, error)
	SetNXPeriod(ctx context.Context, k string, v interface{}, period time.Duration) (bool, error)
	SetExPeriod(ctx context.Context, k string, v interface{}, period time.Duration) error
	MSet(ctx context.Context, values ...interface{}) error
	MSetNX(ctx context.Context, values ...interface{}) error
	Get(ctx context.Context, k string, v interface{}) error
	GetString(ctx context.Context, k string) (string, error)

	// Hash
	HSet(ctx context.Context, key string, value ...interface{}) error
	HGet(ctx context.Context, key string, field string, v interface{}) error
	HGetString(ctx context.Context, key string, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, field string) error

	// Key management
	Del(ctx context.Context, k ...string) error
	Exist(ctx context.Context, k ...string) (bool, error)
	Expire(ctx context.Context, k string, period time.Duration) (bool, error)
	TTL(ctx context.Context, k string) (time.Duration, error)

	// Counter
	Incr(ctx context.Context, k string) (int64, error)
	IncrBy(ctx context.Context, k string, n int64) (int64, error)
	Decr(ctx context.Context, k string) (int64, error)
	DecrBy(ctx context.Context, k string, n int64) (int64, error)

	// List
	LPush(ctx context.Context, k string, values ...interface{}) error
	RPush(ctx context.Context, k string, values ...interface{}) error
	LRange(ctx context.Context, k string, start, stop int64) ([]string, error)
	LLen(ctx context.Context, k string) (int64, error)
	LPop(ctx context.Context, k string) (string, error)
	RPop(ctx context.Context, k string) (string, error)

	// Set
	SAdd(ctx context.Context, k string, members ...interface{}) error
	SMembers(ctx context.Context, k string) ([]string, error)
	SRem(ctx context.Context, k string, members ...interface{}) error
	SIsMember(ctx context.Context, k string, member interface{}) (bool, error)
	SCard(ctx context.Context, k string) (int64, error)

	// Sorted Set
	ZAdd(ctx context.Context, k string, members ...*redis.Z) error
	ZRange(ctx context.Context, k string, start, stop int64) ([]string, error)
	ZRangeWithScores(ctx context.Context, k string, start, stop int64) ([]redis.Z, error)
	ZRevRange(ctx context.Context, k string, start, stop int64) ([]string, error)
	ZRevRangeWithScores(ctx context.Context, k string, start, stop int64) ([]redis.Z, error)
	ZRem(ctx context.Context, k string, members ...interface{}) error
	ZCard(ctx context.Context, k string) (int64, error)
	ZScore(ctx context.Context, k string, member string) (float64, error)
	ZRank(ctx context.Context, k string, member string) (int64, error)
	ZRevRank(ctx context.Context, k string, member string) (int64, error)
}

// Config is a type alias to the shared xredis.Config for backward compatibility.
type Config = xredis.Config

type redisClient struct {
	config    *Config
	redis     redis.Cmdable
	closeFn   func() error
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func NewRedisClient(config Config) (RedisClient, error) {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})
	config.Password = tools.ResolvePasswordFromEnv("REDIS", config.Name, config.Password)

	if len(config.Addrs) == 0 {
		return nil, fmt.Errorf("new redis client failed: addrs must not be empty")
	}

	safeConfig := config
	safeConfig.Password = maskPassword(config.Password)
	logger.Info("**** Create Redis Client **** \n Redis config: %v", tools.ToJson(safeConfig))

	c := &redisClient{
		config: &config,
	}
	password := config.Password
	if config.EnablePasswordEncrypt {
		decrypted, err := tools.AesDecryptRawBase64(password)
		if err != nil {
			return nil, fmt.Errorf("new redis client failed: %w", err)
		}
		password = decrypted
	}
	switch config.Mode {
	case xredis.ClusterMode:
		opts := &redis.ClusterOptions{
			Addrs:        config.Addrs,
			Password:     password,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			IdleTimeout:  idleTimeout(config),
		}
		if maxConnAge := maxConnAge(config); maxConnAge > 0 {
			opts.MaxConnAge = maxConnAge
		}
		cc := redis.NewClusterClient(opts)
		c.redis = cc
		c.closeFn = cc.Close
	default:
		opts := &redis.Options{
			Addr:         config.Addrs[0],
			Password:     password,
			DB:           config.DB,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			IdleTimeout:  idleTimeout(config),
		}
		if maxConnAge := maxConnAge(config); maxConnAge > 0 {
			opts.MaxConnAge = maxConnAge
		}
		sc := redis.NewClient(opts)
		c.redis = sc
		c.closeFn = sc.Close
	}

	timeout, cancelFunc := context.WithTimeout(context.Background(), config.ReadTimeout)
	defer cancelFunc()
	if ping := c.redis.Ping(timeout); ping.Err() != nil {
		return nil, fmt.Errorf("connect redis failed: %w", ping.Err())
	}

	if strings.ToLower(config.EnableMetrics) == "true" {
		c.AddHook(newMetricsHook(c.config))
		c.ctx, c.cancel = context.WithCancel(context.Background())
		startPoolMetrics(c.ctx, c)
	}

	global.DefaultResourceManger.AddWithOrder(c, 1000)
	return c, nil
}

func maskPassword(pwd string) string {
	if pwd == "" {
		return ""
	}
	runes := []rune(pwd)
	if len(runes) <= 4 {
		return "****"
	}
	return string(runes[:2]) + strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-2:])
}

// idleTimeout resolves the idle timeout from v8 (IdleTimeout) or v9 (ConnMaxIdleTime) alias.
// Prefers v9 alias, falls back to v8 name.
func idleTimeout(c Config) time.Duration {
	if c.ConnMaxIdleTime > 0 {
		return c.ConnMaxIdleTime
	}
	return c.IdleTimeout
}

// maxConnAge resolves the max connection age from v8 (MaxConnAge) or v9 (ConnMaxLifetime) alias.
// Prefers v9 alias, falls back to v8 name.
func maxConnAge(c Config) time.Duration {
	if c.ConnMaxLifetime > 0 {
		return c.ConnMaxLifetime
	}
	return c.MaxConnAge
}

func encodingObject(v interface{}) (interface{}, error) {
	if v == nil {
		return v, nil
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Struct, reflect.Ptr, reflect.Map:
		bytes, err := tools.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encoding object failed: %w", err)
		}
		return string(bytes), nil
	case reflect.Slice, reflect.Array:
		if b, ok := v.([]byte); ok {
			return b, nil
		}
		bytes, err := tools.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encoding object failed: %w", err)
		}
		return string(bytes), nil
	default:
		return v, nil
	}
}

func encodingObjects(values []interface{}) ([]interface{}, error) {
	result := make([]interface{}, len(values))
	for i, v := range values {
		encoded, err := encodingObject(v)
		if err != nil {
			return nil, err
		}
		result[i] = encoded
	}
	return result, nil
}

func (c *redisClient) encodingValues(keyWithPrefix bool, values ...interface{}) (interface{}, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("encodingValues: values must not be empty")
	}
	switch v := values[0].(type) {
	case map[string]interface{}:
		m1 := make(map[string]interface{})
		for k, val := range v {
			encoded, err := encodingObject(val)
			if err != nil {
				return nil, err
			}
			if keyWithPrefix {
				m1[c.GetKey(k)] = encoded
			} else {
				m1[k] = encoded
			}
		}
		return m1, nil
	case map[string]string:
		m1 := make(map[string]interface{})
		for k, val := range v {
			if keyWithPrefix {
				m1[c.GetKey(k)] = val
			} else {
				m1[k] = val
			}
		}
		return m1, nil
	case string:
		if len(values)%2 != 0 {
			return nil, fmt.Errorf("encodingValues: key-value pairs must be even, got %d values", len(values))
		}
		copied := make([]interface{}, len(values))
		copy(copied, values)
		for i, elem := range copied {
			if i&1 == 1 {
				encoded, err := encodingObject(elem)
				if err != nil {
					return nil, err
				}
				copied[i] = encoded
			} else {
				if keyWithPrefix {
					s, ok := elem.(string)
					if !ok {
						return nil, fmt.Errorf("encodingValues: key at index %d must be string, got %T", i, elem)
					}
					copied[i] = c.GetKey(s)
				}
			}
		}
		return copied, nil
	default:
		return nil, fmt.Errorf("encodingValues: unsupported values type %T", values[0])
	}
}

func (c *redisClient) Close() {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if err := c.closeFn(); err != nil {
			logger.Error("close redis client failed. err: %s", err.Error())
		}
		logger.Info("redis client closed successfully")
	})
}

func (c *redisClient) Order() int {
	return 1000
}

func (c *redisClient) GetRedis() redis.Cmdable {
	return c.redis
}

func (c *redisClient) AddHook(hook redis.Hook) {
	switch r := c.redis.(type) {
	case *redis.ClusterClient:
		r.AddHook(hook)
	case *redis.Client:
		r.AddHook(hook)
	default:
		logger.Warn("AddHook: unsupported redis.Cmdable type %T, hook not added", r)
	}
}

func (c *redisClient) GetKey(origin string) string {
	if c.config.KeyPrefix != "" {
		return c.config.KeyPrefix + ":" + origin
	}
	return origin
}

// --- String ---

func (c *redisClient) Set(ctx context.Context, k string, v interface{}) error {
	return c.SetPeriod(ctx, k, v, 0)
}

func (c *redisClient) SetPeriod(ctx context.Context, k string, v interface{}, period time.Duration) error {
	encoded, err := encodingObject(v)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, c.GetKey(k), encoded, period).Err()
}

func (c *redisClient) SetNX(ctx context.Context, k string, v interface{}) (bool, error) {
	return c.SetNXPeriod(ctx, k, v, 0)
}

func (c *redisClient) SetNXPeriod(ctx context.Context, k string, v interface{}, period time.Duration) (bool, error) {
	encoded, err := encodingObject(v)
	if err != nil {
		return false, err
	}
	return c.redis.SetNX(ctx, c.GetKey(k), encoded, period).Result()
}

func (c *redisClient) SetExPeriod(ctx context.Context, k string, v interface{}, period time.Duration) error {
	if period <= 0 {
		return fmt.Errorf("SetExPeriod: period must be positive, got %v", period)
	}
	encoded, err := encodingObject(v)
	if err != nil {
		return err
	}
	return c.redis.SetEX(ctx, c.GetKey(k), encoded, period).Err()
}

func (c *redisClient) MSet(ctx context.Context, values ...interface{}) error {
	encoded, err := c.encodingValues(true, values...)
	if err != nil {
		return err
	}
	return c.redis.MSet(ctx, encoded).Err()
}

func (c *redisClient) MSetNX(ctx context.Context, values ...interface{}) error {
	encoded, err := c.encodingValues(true, values...)
	if err != nil {
		return err
	}
	return c.redis.MSetNX(ctx, encoded).Err()
}

func (c *redisClient) Get(ctx context.Context, k string, v interface{}) error {
	bytes, err := c.redis.Get(ctx, c.GetKey(k)).Bytes()
	if err != nil {
		return wrapNilError(err)
	}
	return tools.Unmarshal(bytes, v)
}

func (c *redisClient) GetString(ctx context.Context, k string) (string, error) {
	result, err := c.redis.Get(ctx, c.GetKey(k)).Result()
	if err != nil {
		return "", wrapNilError(err)
	}
	return result, nil
}

// --- Hash ---

func (c *redisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	encoded, err := c.encodingValues(false, values...)
	if err != nil {
		return err
	}
	return c.redis.HSet(ctx, c.GetKey(key), encoded).Err()
}

func (c *redisClient) HGet(ctx context.Context, key string, field string, v interface{}) error {
	bytes, err := c.redis.HGet(ctx, c.GetKey(key), field).Bytes()
	if err != nil {
		return wrapNilError(err)
	}
	return tools.Unmarshal(bytes, v)
}

func (c *redisClient) HGetString(ctx context.Context, key string, field string) (string, error) {
	result, err := c.redis.HGet(ctx, c.GetKey(key), field).Result()
	if err != nil {
		return "", wrapNilError(err)
	}
	return result, nil
}

// HGetAll returns all fields and values of the hash stored at key.
// If the key does not exist, an empty map is returned (not an error).
// This differs from Get/HGet which return ErrNil for missing keys.
func (c *redisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.redis.HGetAll(ctx, c.GetKey(key)).Result()
}

func (c *redisClient) HDel(ctx context.Context, key string, field string) error {
	return c.redis.HDel(ctx, c.GetKey(key), field).Err()
}

// --- Key management ---

func (c *redisClient) Del(ctx context.Context, k ...string) error {
	if len(k) == 0 {
		return nil
	}
	var keys []string
	for _, t := range k {
		keys = append(keys, c.GetKey(t))
	}
	return c.redis.Del(ctx, keys...).Err()
}

func (c *redisClient) Exist(ctx context.Context, k ...string) (bool, error) {
	if len(k) == 0 {
		return false, nil
	}
	var keys []string
	for _, t := range k {
		keys = append(keys, c.GetKey(t))
	}
	v, err := c.redis.Exists(ctx, keys...).Result()
	if err != nil {
		return false, err
	}
	return v > 0, nil
}

func (c *redisClient) Expire(ctx context.Context, k string, period time.Duration) (bool, error) {
	return c.redis.Expire(ctx, c.GetKey(k), period).Result()
}

// TTL returns the remaining time to live of a key.
// Returns -2 if the key does not exist, -1 if the key exists but has no expiry.
func (c *redisClient) TTL(ctx context.Context, k string) (time.Duration, error) {
	return c.redis.TTL(ctx, c.GetKey(k)).Result()
}

// --- Counter ---

func (c *redisClient) Incr(ctx context.Context, k string) (int64, error) {
	return c.redis.Incr(ctx, c.GetKey(k)).Result()
}

func (c *redisClient) IncrBy(ctx context.Context, k string, n int64) (int64, error) {
	return c.redis.IncrBy(ctx, c.GetKey(k), n).Result()
}

func (c *redisClient) Decr(ctx context.Context, k string) (int64, error) {
	return c.redis.Decr(ctx, c.GetKey(k)).Result()
}

func (c *redisClient) DecrBy(ctx context.Context, k string, n int64) (int64, error) {
	return c.redis.DecrBy(ctx, c.GetKey(k), n).Result()
}

// --- List ---

func (c *redisClient) LPush(ctx context.Context, k string, values ...interface{}) error {
	encoded, err := encodingObjects(values)
	if err != nil {
		return err
	}
	return c.redis.LPush(ctx, c.GetKey(k), encoded...).Err()
}

func (c *redisClient) RPush(ctx context.Context, k string, values ...interface{}) error {
	encoded, err := encodingObjects(values)
	if err != nil {
		return err
	}
	return c.redis.RPush(ctx, c.GetKey(k), encoded...).Err()
}

// LRange returns the specified elements of the list stored at key.
// If the key does not exist, an empty slice is returned (not an error).
// This differs from LPop/RPop which return ErrNil for missing keys.
func (c *redisClient) LRange(ctx context.Context, k string, start, stop int64) ([]string, error) {
	return c.redis.LRange(ctx, c.GetKey(k), start, stop).Result()
}

// LLen returns the length of the list stored at key.
// If the key does not exist, 0 is returned (not an error).
func (c *redisClient) LLen(ctx context.Context, k string) (int64, error) {
	return c.redis.LLen(ctx, c.GetKey(k)).Result()
}

func (c *redisClient) LPop(ctx context.Context, k string) (string, error) {
	result, err := c.redis.LPop(ctx, c.GetKey(k)).Result()
	if err != nil {
		return "", wrapNilError(err)
	}
	return result, nil
}

func (c *redisClient) RPop(ctx context.Context, k string) (string, error) {
	result, err := c.redis.RPop(ctx, c.GetKey(k)).Result()
	if err != nil {
		return "", wrapNilError(err)
	}
	return result, nil
}

// --- Set ---

func (c *redisClient) SAdd(ctx context.Context, k string, members ...interface{}) error {
	encoded, err := encodingObjects(members)
	if err != nil {
		return err
	}
	return c.redis.SAdd(ctx, c.GetKey(k), encoded...).Err()
}

// SMembers returns all members of the set stored at key.
// If the key does not exist, an empty slice is returned (not an error).
func (c *redisClient) SMembers(ctx context.Context, k string) ([]string, error) {
	return c.redis.SMembers(ctx, c.GetKey(k)).Result()
}

func (c *redisClient) SRem(ctx context.Context, k string, members ...interface{}) error {
	encoded, err := encodingObjects(members)
	if err != nil {
		return err
	}
	return c.redis.SRem(ctx, c.GetKey(k), encoded...).Err()
}

func (c *redisClient) SIsMember(ctx context.Context, k string, member interface{}) (bool, error) {
	encoded, err := encodingObject(member)
	if err != nil {
		return false, err
	}
	return c.redis.SIsMember(ctx, c.GetKey(k), encoded).Result()
}

func (c *redisClient) SCard(ctx context.Context, k string) (int64, error) {
	return c.redis.SCard(ctx, c.GetKey(k)).Result()
}

// --- Sorted Set ---

// ZAdd adds members to a sorted set.
// Note: redis.Z.Member is not automatically JSON-encoded. If you need to store
// struct/map/slice as member, you must manually serialize it before passing to ZAdd.
func (c *redisClient) ZAdd(ctx context.Context, k string, members ...*redis.Z) error {
	return c.redis.ZAdd(ctx, c.GetKey(k), members...).Err()
}

func (c *redisClient) ZRange(ctx context.Context, k string, start, stop int64) ([]string, error) {
	return c.redis.ZRange(ctx, c.GetKey(k), start, stop).Result()
}

func (c *redisClient) ZRangeWithScores(ctx context.Context, k string, start, stop int64) ([]redis.Z, error) {
	return c.redis.ZRangeWithScores(ctx, c.GetKey(k), start, stop).Result()
}

func (c *redisClient) ZRevRange(ctx context.Context, k string, start, stop int64) ([]string, error) {
	return c.redis.ZRevRange(ctx, c.GetKey(k), start, stop).Result()
}

func (c *redisClient) ZRevRangeWithScores(ctx context.Context, k string, start, stop int64) ([]redis.Z, error) {
	return c.redis.ZRevRangeWithScores(ctx, c.GetKey(k), start, stop).Result()
}

// ZRem removes members from a sorted set.
// Note: members are not automatically JSON-encoded, consistent with ZAdd behavior.
// If you stored JSON-serialized members via ZAdd, pass the same serialized values here.
func (c *redisClient) ZRem(ctx context.Context, k string, members ...interface{}) error {
	return c.redis.ZRem(ctx, c.GetKey(k), members...).Err()
}

func (c *redisClient) ZCard(ctx context.Context, k string) (int64, error) {
	return c.redis.ZCard(ctx, c.GetKey(k)).Result()
}

func (c *redisClient) ZScore(ctx context.Context, k string, member string) (float64, error) {
	result, err := c.redis.ZScore(ctx, c.GetKey(k), member).Result()
	if err != nil {
		return 0, wrapNilError(err)
	}
	return result, nil
}

// ZRank returns the rank of member in the sorted set, with scores ordered low to high.
// Returns ErrNil if the key or member does not exist.
// Note: Redis does not distinguish between "key not found" and "member not found";
// both cases return nil, which is converted to ErrNil.
func (c *redisClient) ZRank(ctx context.Context, k string, member string) (int64, error) {
	result, err := c.redis.ZRank(ctx, c.GetKey(k), member).Result()
	if err != nil {
		return 0, wrapNilError(err)
	}
	return result, nil
}

// ZRevRank returns the rank of member in the sorted set, with scores ordered high to low.
// Returns ErrNil if the key or member does not exist.
// Note: Redis does not distinguish between "key not found" and "member not found";
// both cases return nil, which is converted to ErrNil.
func (c *redisClient) ZRevRank(ctx context.Context, k string, member string) (int64, error) {
	result, err := c.redis.ZRevRank(ctx, c.GetKey(k), member).Result()
	if err != nil {
		return 0, wrapNilError(err)
	}
	return result, nil
}

func wrapNilError(err error) error {
	if errors.Is(err, redis.Nil) {
		return ErrNil
	}
	return err
}
