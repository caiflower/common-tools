package v1

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/go-redis/redis/v8"
)

const (
	ClusterMode = "cluster"
)

type RedisClient interface {
	GetRedis() redis.Cmdable
	Set(k string, v interface{}) error
	SetPeriod(k string, v interface{}, period time.Duration) error
	SetNX(k string, v interface{}) error
	SetNXPeriod(k string, v interface{}, period time.Duration) error
	SetEx(k string, v interface{}) error
	SetEXPeriod(k string, v interface{}, period time.Duration) error
	MSet(values ...interface{}) error
	MSetNX(values ...interface{}) error
	Get(k string, v interface{}) error
	GetString(k string) (string, error)
	HSet(key string, value ...interface{}) error
	HGet(key string, field string, v interface{}) error
	HGetString(key string, field string) (string, error)
	HGetAll(key string) (map[string]string, error)
	HDel(key string, field string) error
	Del(k ...string) error
	Expire(k string, period time.Duration) (bool, error)
	Exist(k ...string) (bool, error)
}

type Config struct {
	Mode                  string        `yaml:"mode"`
	Addrs                 []string      `yaml:"addrs"`
	Password              string        `yaml:"password"`
	EnablePasswordEncrypt bool          `yaml:"enable_password_encrypt"`
	DB                    int           `yaml:"db"`
	ReadTimeout           time.Duration `yaml:"readTimeout" default:"10s"`
	WriteTimeout          time.Duration `yaml:"writeTimeout" default:"20s"`
	PoolSize              int           `yaml:"poolSize"`
	MinIdleConns          int           `yaml:"minIdleConns" default:"20"`
	MaxConnAge            time.Duration `yaml:"maxConnAge" default:"80s"`
}

type redisClient struct {
	config        *Config
	client        *redis.Client
	clusterClient *redis.ClusterClient
}

func NewRedisClient(config Config) RedisClient {
	_ = tools.DoTagFunc(&config, nil, []func(reflect.StructField, reflect.Value, interface{}) error{tools.SetDefaultValueIfNil})

	logger.Info("**** Create Redis Client **** \n Redis config: %v", tools.ToJson(config))
	c := &redisClient{
		config: &config,
	}
	password := config.Password
	if config.EnablePasswordEncrypt {
		_tmpPassword, err := tools.AesDecryptBase64(password)
		if err != nil {
			panic(fmt.Sprintf("new redis client failed. err: %v", err))
		}
		password = _tmpPassword
	}
	switch config.Mode {
	case ClusterMode:
		c.clusterClient = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.Addrs,
			Password:     password,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			MaxConnAge:   config.MaxConnAge,
		})
	default:
		c.client = redis.NewClient(&redis.Options{
			Addr:         config.Addrs[0],
			Password:     password,
			DB:           config.DB,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			MaxConnAge:   config.MaxConnAge,
		})
	}

	global.DefaultResourceManger.Add(c)
	return c
}

func EncodingObject(v interface{}) interface{} {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Struct, reflect.Ptr:
		bytes, _ := tools.Marshal(v)
		return string(bytes)
	default:
		return v
	}
}

func EncodingValues(values ...interface{}) interface{} {
	switch values[0].(type) {
	case map[string]interface{}:
		m := values[0].(map[string]interface{})
		for k, v := range m {
			m[k] = EncodingObject(v)
		}
		return values[0]
	case map[string]string:
		m := values[0].(map[string]string)
		for k, v := range m {
			m[k] = EncodingObject(v).(string)
		}
		return values[0]
	case string:
		for i, v := range values {
			if i&1 == 1 {
				values[i] = EncodingObject(v)
			}
		}
		return values
	default:
		fmt.Println(reflect.TypeOf(values[0]).Kind())
		return values
	}

}

func (c *redisClient) Close() {
	switch c.config.Mode {
	case ClusterMode:
		c.clusterClient.Close()
	default:
		err := c.client.Close()
		if err != nil {
			logger.Error("close redis client failed. err: %s", err.Error())
		}
	}
}

func (c *redisClient) GetRedis() redis.Cmdable {
	switch c.config.Mode {
	case ClusterMode:
		return c.clusterClient
	default:
		return c.client
	}
}

func (c *redisClient) Set(k string, v interface{}) error {
	return c.SetPeriod(k, v, 0)
}

func (c *redisClient) SetPeriod(k string, v interface{}, period time.Duration) error {
	return c.GetRedis().Set(context.Background(), k, EncodingObject(v), period).Err()
}

func (c *redisClient) SetNX(k string, v interface{}) error {
	return c.SetNXPeriod(k, v, 0)
}

func (c *redisClient) SetNXPeriod(k string, v interface{}, period time.Duration) error {
	return c.GetRedis().SetNX(context.Background(), k, EncodingObject(v), period).Err()
}

func (c *redisClient) SetEx(k string, v interface{}) error {
	return c.SetEXPeriod(k, v, 0)
}

func (c *redisClient) SetEXPeriod(k string, v interface{}, period time.Duration) error {
	return c.GetRedis().SetEX(context.Background(), k, EncodingObject(v), period).Err()
}

func (c *redisClient) Get(k string, v interface{}) error {
	if bytes, err := c.GetRedis().Get(context.Background(), k).Bytes(); err != nil {
		return err
	} else {
		return tools.Unmarshal(bytes, v)
	}
}

func (c *redisClient) GetString(k string) (string, error) {
	return c.GetRedis().Get(context.Background(), k).Result()
}

func (c *redisClient) Del(k ...string) error {
	return c.GetRedis().Del(context.Background(), k...).Err()
}

func (c *redisClient) Exist(k ...string) (bool, error) {
	if v, err := c.GetRedis().Exists(context.Background(), k...).Result(); err != nil {
		return false, err
	} else {
		return v == 1, nil
	}
}

func (c *redisClient) Expire(k string, period time.Duration) (bool, error) {
	return c.GetRedis().Expire(context.Background(), k, period).Result()
}

func (c *redisClient) MSet(values ...interface{}) error {
	return c.GetRedis().MSet(context.Background(), EncodingValues(values...)).Err()
}

func (c *redisClient) MSetNX(values ...interface{}) error {
	return c.GetRedis().MSetNX(context.Background(), EncodingValues(values...)).Err()
}

func (c *redisClient) HSet(key string, values ...interface{}) error {
	return c.GetRedis().HSet(context.Background(), key, EncodingValues(values...)).Err()
}

func (c *redisClient) HGet(key string, field string, v interface{}) error {
	if bytes, err := c.GetRedis().HGet(context.Background(), key, field).Bytes(); err != nil {
		return err
	} else {
		return tools.Unmarshal(bytes, v)
	}
}

func (c *redisClient) HGetString(key string, field string) (string, error) {
	return c.GetRedis().HGet(context.Background(), key, field).Result()
}

func (c *redisClient) HGetAll(key string) (map[string]string, error) {
	return c.GetRedis().HGetAll(context.Background(), key).Result()
}

func (c *redisClient) HDel(key string, field string) error {
	return c.GetRedis().HDel(context.Background(), key, field).Err()
}
