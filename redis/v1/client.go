package redisv1

import (
	"context"
	"fmt"
	"reflect"
	"time"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"

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
	AddHook(hook redis.Hook)
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
	GetKey(k string) string // get key with keyPrefix
}

type Config struct {
	Mode                  string        `yaml:"mode" json:"mode"`
	Addrs                 []string      `yaml:"addrs" json:"addrs"`
	Password              string        `yaml:"password" json:"password"`
	EnablePasswordEncrypt bool          `yaml:"enablePasswordEncrypt" json:"enablePasswordEncrypt"`
	DB                    int           `yaml:"db" json:"db"`
	ReadTimeout           time.Duration `yaml:"readTimeout" default:"10s" json:"readTimeout"`
	WriteTimeout          time.Duration `yaml:"writeTimeout" default:"20s" json:"writeTimeout"`
	PoolSize              int           `yaml:"poolSize" json:"poolSize"`
	MinIdleConns          int           `yaml:"minIdleConns" default:"20" json:"minIdleConns"`
	MaxConnAge            time.Duration `yaml:"maxConnAge" default:"80s" json:"maxConnAge"`
	KeyPrefix             string        `yaml:"keyPrefix" json:"keyPrefix"`
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
		_tmpPassword, err := tools.AesDecryptRawBase64(password)
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

	if ping := c.GetRedis().Ping(context.Background()); ping.Err() != nil {
		panic("connect redis failed. Error: " + ping.Err().Error())
	}

	global.DefaultResourceManger.Add(c)
	return c
}

func encodingObject(v interface{}) interface{} {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Struct, reflect.Ptr:
		bytes, _ := tools.Marshal(v)
		return string(bytes)
	default:
		return v
	}
}

// encodingValues
func (c *redisClient) encodingValues(keyWithPrefix bool, values ...interface{}) interface{} {
	switch values[0].(type) {
	case map[string]interface{}:
		m := values[0].(map[string]interface{})
		m1 := make(map[string]interface{})
		for k, v := range m {
			if keyWithPrefix {
				m1[c.GetKey(k)] = encodingObject(v)
			} else {
				m1[k] = encodingObject(v)
			}
		}
		return m1
	case map[string]string:
		m := values[0].(map[string]string)
		m1 := make(map[string]interface{})
		for k, v := range m {
			if keyWithPrefix {
				m1[c.GetKey(k)] = encodingObject(v)
			} else {
				m1[k] = encodingObject(v)
			}
		}
		return m1
	case string:
		for i, v := range values {
			if i&1 == 1 {
				values[i] = encodingObject(v)
			} else {
				if keyWithPrefix {
					values[i] = c.GetKey(values[i].(string))
				}
			}
		}
		return values
	default:
		fmt.Println(reflect.TypeOf(values[0]).Kind())
		return values
	}

}

func (c *redisClient) Close() {
	var err error
	switch c.config.Mode {
	case ClusterMode:
		err = c.clusterClient.Close()
	default:
		err = c.client.Close()
	}

	if err != nil {
		logger.Error("close redis client failed. err: %s", err.Error())
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

func (c *redisClient) AddHook(hook redis.Hook) {
	switch c.config.Mode {
	case ClusterMode:
		c.clusterClient.AddHook(hook)
	default:
		c.client.AddHook(hook)
	}
}

func (c *redisClient) Set(k string, v interface{}) error {
	return c.SetPeriod(k, v, 0)
}

func (c *redisClient) SetPeriod(k string, v interface{}, period time.Duration) error {
	return c.GetRedis().Set(GetContext(), c.GetKey(k), encodingObject(v), period).Err()
}

func (c *redisClient) SetNX(k string, v interface{}) error {
	return c.SetNXPeriod(k, v, 0)
}

func (c *redisClient) SetNXPeriod(k string, v interface{}, period time.Duration) error {
	return c.GetRedis().SetNX(GetContext(), c.GetKey(k), encodingObject(v), period).Err()
}

func (c *redisClient) SetEx(k string, v interface{}) error {
	return c.SetEXPeriod(k, v, 0)
}

func (c *redisClient) SetEXPeriod(k string, v interface{}, period time.Duration) error {
	return c.GetRedis().SetEX(GetContext(), c.GetKey(k), encodingObject(v), period).Err()
}

func (c *redisClient) Get(k string, v interface{}) error {
	if bytes, err := c.GetRedis().Get(GetContext(), c.GetKey(k)).Bytes(); err != nil {
		return err
	} else {
		return tools.Unmarshal(bytes, v)
	}
}

func (c *redisClient) GetString(k string) (string, error) {
	return c.GetRedis().Get(GetContext(), c.GetKey(k)).Result()
}

func (c *redisClient) Del(k ...string) error {
	var keys []string
	for _, t := range k {
		keys = append(keys, c.GetKey(t))
	}
	return c.GetRedis().Del(GetContext(), keys...).Err()
}

func (c *redisClient) Exist(k ...string) (bool, error) {
	var keys []string
	for _, t := range k {
		keys = append(keys, c.GetKey(t))
	}
	if v, err := c.GetRedis().Exists(GetContext(), keys...).Result(); err != nil {
		return false, err
	} else {
		return v == 1, nil
	}
}

func (c *redisClient) Expire(k string, period time.Duration) (bool, error) {
	return c.GetRedis().Expire(GetContext(), c.GetKey(k), period).Result()
}

func (c *redisClient) MSet(values ...interface{}) error {
	return c.GetRedis().MSet(GetContext(), c.encodingValues(true, values...)).Err()
}

func (c *redisClient) MSetNX(values ...interface{}) error {
	return c.GetRedis().MSetNX(GetContext(), c.encodingValues(true, values...)).Err()
}

func (c *redisClient) HSet(key string, values ...interface{}) error {
	return c.GetRedis().HSet(GetContext(), c.GetKey(key), c.encodingValues(false, values...)).Err()
}

func (c *redisClient) HGet(key string, field string, v interface{}) error {
	if bytes, err := c.GetRedis().HGet(GetContext(), c.GetKey(key), field).Bytes(); err != nil {
		return err
	} else {
		return tools.Unmarshal(bytes, v)
	}
}

func (c *redisClient) HGetString(key string, field string) (string, error) {
	return c.GetRedis().HGet(GetContext(), c.GetKey(key), field).Result()
}

func (c *redisClient) HGetAll(key string) (map[string]string, error) {
	return c.GetRedis().HGetAll(GetContext(), c.GetKey(key)).Result()
}

func (c *redisClient) HDel(key string, field string) error {
	return c.GetRedis().HDel(GetContext(), c.GetKey(key), field).Err()
}

func (c *redisClient) GetKey(origin string) string {
	if c.config.KeyPrefix != "" {
		return c.config.KeyPrefix + origin
	}
	return origin
}

// GetContext getContext with traceId
func GetContext() context.Context {
	return context.WithValue(golocalv1.GetContext(), "traceId", golocalv1.GetTraceID())
}
