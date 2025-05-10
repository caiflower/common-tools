package redisv1

import (
	"github.com/google/go-cmp/cmp"
	"testing"
	"time"
)

type TestObject struct {
	Name string
	Age  int
}

func commonTestcase(client RedisClient) {
	if err := client.Set("test", "test"); err != nil {
		panic(err)
	}

	if getString, err := client.GetString("test"); err != nil {
		panic(err)
	} else {
		assert(getString == "test")
	}

	if err := client.Del("test"); err != nil {
		panic(err)
	}

	object := &TestObject{Age: 1, Name: "testObject"}
	if err := client.Set("object", object); err != nil {
		panic(err)
	}

	object1 := &TestObject{}
	if err := client.Get("object", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if err := client.Del("object"); err != nil {
		panic(err)
	}

	if err := client.SetPeriod("objectTTL", &TestObject{Age: 1, Name: "testObject"}, time.Second*60); err != nil {
		panic(err)
	}

	if err := client.HSet("ObjectH", "key", object); err != nil {
		panic(err)
	}

	if err := client.HSet("ObjectH", map[string]interface{}{"key1": object}); err != nil {
		panic(err)
	}

	if err := client.HSet("ObjectH", map[string]string{"key2": "key2string"}); err != nil {
		panic(err)
	}

	if err := client.HGet("ObjectH", "key", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if err := client.HGet("ObjectH", "key1", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if v, err := client.HGetString("ObjectH", "key2"); err != nil {
		panic(err)
	} else {
		assert(v == "key2string")
	}

	if err := client.Del("ObjectH"); err != nil {
		panic(err)
	}

	m := make(map[string]interface{})
	m["mSetKey1"] = "MSetValue1"
	m["mSetKey2"] = object
	if err := client.MSet(m); err != nil {
		panic(err)
	}

	if getString, err := client.GetString("mSetKey1"); err != nil {
		panic(err)
	} else {
		assert(getString == "MSetValue1")
	}

	if err := client.Get("mSetKey2", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if err := client.Del("mSetKey1", "mSetKey2"); err != nil {
		panic(err)
	}

	m1 := make(map[string]string)
	m1["mSetKey1"] = "MSetValue1"
	m1["mSetKey2"] = "MSetValue2"

	if err := client.MSet(m1); err != nil {
		panic(err)
	}

	if getString, err := client.GetString("mSetKey1"); err != nil {
		panic(err)
	} else {
		assert(getString == "MSetValue1")
	}

	if getString, err := client.GetString("mSetKey1"); err != nil {
		panic(err)
	} else {
		assert(getString == "MSetValue1")
	}

	if err := client.Del("mSetKey1", "mSetKey2"); err != nil {
		panic(err)
	}
}

func TestNewRedisClient(t *testing.T) {
	client := NewRedisClient(Config{
		Addrs:    []string{"redis-headless.svc.app.cluster.local:6379"},
		Password: "",
		DB:       1,
	})
	if client == nil {
		panic("client init failed")
	}

	commonTestcase(client)
}

func TestNewRedisClientWithPrefix(t *testing.T) {
	client := NewRedisClient(Config{
		Addrs:     []string{"redis-headless.svc.app.cluster.local:6379"},
		Password:  "",
		DB:        1,
		KeyPrefix: "test:",
	})
	if client == nil {
		panic("client init failed")
	}

	commonTestcase(client)
}

func assert(success bool) {
	if !success {
		panic("test failed.")
	}
}
