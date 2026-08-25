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

package v2

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	xredis "github.com/caiflower/common-tools/redis"
	"github.com/redis/go-redis/v9"
)

// Config is a type alias to the shared xredis.Config for backward compatibility.
type Config = xredis.Config

// Cmdable extends redis.Cmdable with a Key helper for applying the configured key prefix.
// Usage: client.Cmd().Set(ctx, client.Cmd().Key("foo"), "bar", 5*time.Minute).Err()
type Cmdable interface {
	redis.Cmdable
	// Key applies the configured KeyPrefix to the given key.
	Key(key string) string
}

// RedisClient provides a minimal wrapper around go-redis v9.
// It only manages connection lifecycle, key prefixing, and metrics injection.
// All Redis commands are accessed directly through the native Cmdable interface.
type RedisClient interface {
	// Cmd returns the underlying go-redis v9 Cmdable with key prefix support.
	Cmd() Cmdable

	// GetRedis returns the raw underlying redis.Cmdable without key prefix wrapping.
	// Use this for operations where keys already include the prefix (e.g., SCAN results).
	GetRedis() redis.Cmdable

	// AddHook registers a Hook on the underlying client.
	AddHook(hook redis.Hook)

	// Close gracefully shuts down the Redis connection and background metrics.
	Close()
}

type redisClient struct {
	config    *Config
	redis     redis.Cmdable
	closeFn   func() error
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// redisCmd wraps redis.Cmdable with key prefix support.
type redisCmd struct {
	redis.Cmdable
	config *Config
}

func (c *redisCmd) Key(key string) string {
	if c.config.KeyPrefix != "" {
		return c.config.KeyPrefix + ":" + key
	}
	return key
}

// NewRedisClient creates a new Redis client using go-redis v9.
func NewRedisClient(config Config) (RedisClient, error) {
	_ = tools.DoTagFunc(&config, []tools.FnObj{{Fn: tools.SetDefaultValueIfNil}})
	config.Password = tools.ResolvePasswordFromEnv("REDIS", config.Name, config.Password)

	if len(config.Addrs) == 0 {
		return nil, fmt.Errorf("new redis client failed: addrs must not be empty")
	}

	safeConfig := config
	safeConfig.Password = maskPassword(config.Password)
	logger.Info("**** Create Redis Client (v2/go-redis-v9) **** \n Redis config: %v", tools.ToJson(safeConfig))

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
			Addrs:           config.Addrs,
			Password:        password,
			ReadTimeout:     config.ReadTimeout,
			WriteTimeout:    config.WriteTimeout,
			PoolSize:        config.PoolSize,
			MinIdleConns:    config.MinIdleConns,
			ConnMaxIdleTime: connMaxIdleTime(config),
			ConnMaxLifetime: connMaxLifetime(config),
			DisableIdentity: config.DisableIdentity,
		}
		cc := redis.NewClusterClient(opts)
		c.redis = cc
		c.closeFn = cc.Close
	default:
		opts := &redis.Options{
			Addr:            config.Addrs[0],
			Password:        password,
			DB:              config.DB,
			ReadTimeout:     config.ReadTimeout,
			WriteTimeout:    config.WriteTimeout,
			PoolSize:        config.PoolSize,
			MinIdleConns:    config.MinIdleConns,
			ConnMaxIdleTime: connMaxIdleTime(config),
			ConnMaxLifetime: connMaxLifetime(config),
			DisableIdentity: config.DisableIdentity,
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
		metricsCtx, cancel := context.WithCancel(context.Background())
		c.cancel = cancel
		startPoolMetrics(metricsCtx, c)
	}

	global.DefaultResourceManger.AddWithOrder(c, 1000)
	return c, nil
}

// connMaxLifetime returns ConnMaxLifetime if set, otherwise falls back to the legacy MaxConnAge.
func connMaxLifetime(c Config) time.Duration {
	if c.ConnMaxLifetime > 0 {
		return c.ConnMaxLifetime
	}
	return c.MaxConnAge
}

// connMaxIdleTime returns ConnMaxIdleTime if set, otherwise falls back to the legacy IdleTimeout.
func connMaxIdleTime(c Config) time.Duration {
	if c.ConnMaxIdleTime > 0 {
		return c.ConnMaxIdleTime
	}
	return c.IdleTimeout
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

// Cmd returns a Cmdable with Key prefix support.
func (c *redisClient) Cmd() Cmdable {
	return &redisCmd{Cmdable: c.redis, config: c.config}
}

// GetRedis returns the raw underlying redis.Cmdable without key prefix wrapping.
func (c *redisClient) GetRedis() redis.Cmdable {
	return c.redis
}

// AddHook registers a Hook on the underlying Redis client.
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

// Close gracefully shuts down the connection and stops background metrics collection.
func (c *redisClient) Close() {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if err := c.closeFn(); err != nil {
			logger.Error("close redis client failed. err: %s", err.Error())
		} else {
			logger.Info("redis v2 client closed successfully")
		}
	})
}

func (c *redisClient) Order() int {
	return 1000
}
