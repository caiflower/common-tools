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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type TestObject struct {
	Name string
	Age  int
}

func commonTestcase(client RedisClient) {
	ctx := context.Background()

	if err := client.Set(ctx, "test", "test"); err != nil {
		panic(err)
	}

	if getString, err := client.GetString(ctx, "test"); err != nil {
		panic(err)
	} else {
		assert(getString == "test")
	}

	if err := client.Del(ctx, "test"); err != nil {
		panic(err)
	}

	object := &TestObject{Age: 1, Name: "testObject"}
	if err := client.Set(ctx, "object", object); err != nil {
		panic(err)
	}

	object1 := &TestObject{}
	if err := client.Get(ctx, "object", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if err := client.Del(ctx, "object"); err != nil {
		panic(err)
	}

	if err := client.SetPeriod(ctx, "objectTTL", &TestObject{Age: 1, Name: "testObject"}, time.Second*60); err != nil {
		panic(err)
	}

	if err := client.HSet(ctx, "ObjectH", "key", object); err != nil {
		panic(err)
	}

	if err := client.HSet(ctx, "ObjectH", map[string]interface{}{"key1": object}); err != nil {
		panic(err)
	}

	if err := client.HSet(ctx, "ObjectH", map[string]string{"key2": "key2string"}); err != nil {
		panic(err)
	}

	if err := client.HGet(ctx, "ObjectH", "key", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if err := client.HGet(ctx, "ObjectH", "key1", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if v, err := client.HGetString(ctx, "ObjectH", "key2"); err != nil {
		panic(err)
	} else {
		assert(v == "key2string")
	}

	if err := client.Del(ctx, "ObjectH"); err != nil {
		panic(err)
	}

	m := make(map[string]interface{})
	m["mSetKey1"] = "MSetValue1"
	m["mSetKey2"] = object
	if err := client.MSet(ctx, m); err != nil {
		panic(err)
	}

	if getString, err := client.GetString(ctx, "mSetKey1"); err != nil {
		panic(err)
	} else {
		assert(getString == "MSetValue1")
	}

	if err := client.Get(ctx, "mSetKey2", object1); err != nil {
		panic(err)
	} else {
		assert(cmp.Equal(object, object1))
	}

	if err := client.Del(ctx, "mSetKey1", "mSetKey2"); err != nil {
		panic(err)
	}

	m1 := make(map[string]string)
	m1["mSetKey1"] = "MSetValue1"
	m1["mSetKey2"] = "MSetValue2"

	if err := client.MSet(ctx, m1); err != nil {
		panic(err)
	}

	if getString, err := client.GetString(ctx, "mSetKey1"); err != nil {
		panic(err)
	} else {
		assert(getString == "MSetValue1")
	}

	if getString, err := client.GetString(ctx, "mSetKey1"); err != nil {
		panic(err)
	} else {
		assert(getString == "MSetValue1")
	}

	if err := client.Del(ctx, "mSetKey1", "mSetKey2"); err != nil {
		panic(err)
	}
}

func TestNewRedisClient(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Skip()
		}
	}()

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
	defer func() {
		if err := recover(); err != nil {
			t.Skip()
		}
	}()

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
