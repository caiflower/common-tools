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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-cmp/cmp"
	tassert "github.com/stretchr/testify/assert"
	trequire "github.com/stretchr/testify/require"
)

type TestObject struct {
	Name string
	Age  int
}

func setupTestClient(t *testing.T, opts ...func(*Config)) RedisClient {
	t.Helper()
	mr := miniredis.RunT(t)

	cfg := Config{
		Addrs: []string{mr.Addr()},
		DB:    0,
	}
	for _, o := range opts {
		o(&cfg)
	}

	client, err := NewRedisClient(cfg)
	trequire.NoError(t, err, "NewRedisClient should succeed with miniredis")
	return client
}

func withKeyPrefix(prefix string) func(*Config) {
	return func(c *Config) { c.KeyPrefix = prefix }
}

func withMetrics() func(*Config) {
	return func(c *Config) { c.EnableMetrics = ptrBool(true) }
}

func ptrBool(v bool) *bool { return &v }

func TestNewRedisClient_BasicSetGet(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "test", "test"), "Set string value")

	got, err := client.GetString(ctx, "test")
	trequire.NoError(t, err, "GetString should succeed")
	tassert.Equal(t, "test", got)

	trequire.NoError(t, client.Del(ctx, "test"), "Del should succeed")
}

func TestNewRedisClient_SetGetStruct(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := &TestObject{Age: 1, Name: "testObject"}
	trequire.NoError(t, client.Set(ctx, "object", original), "Set struct value")

	got := &TestObject{}
	trequire.NoError(t, client.Get(ctx, "object", got), "Get struct value")
	tassert.True(t, cmp.Equal(original, got), "round-tripped struct should match")

	trequire.NoError(t, client.Del(ctx, "object"), "Del should succeed")
}

func TestNewRedisClient_SetPeriod(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.SetPeriod(ctx, "objectTTL", &TestObject{Age: 1, Name: "testObject"}, time.Second*60), "SetPeriod should succeed")
}

func TestNewRedisClient_SetNX(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	ok, err := client.SetNX(ctx, "nxkey", "value1")
	trequire.NoError(t, err, "SetNX first time should not return error")
	tassert.True(t, ok, "SetNX first time should return true")

	ok, err = client.SetNX(ctx, "nxkey", "value2")
	trequire.NoError(t, err, "SetNX on existing key should not return error")
	tassert.False(t, ok, "SetNX on existing key should return false")

	got, err := client.GetString(ctx, "nxkey")
	trequire.NoError(t, err, "GetString nxkey")
	tassert.Equal(t, "value1", got, "value should remain as value1, not overwritten by value2")
}

func TestNewRedisClient_SetExPeriod(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.SetExPeriod(ctx, "exkey", "value", time.Second*60), "SetExPeriod should succeed")
}

func TestNewRedisClient_SetExPeriodZeroTTL(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.SetExPeriod(ctx, "exkey", "value", 0)
	tassert.Error(t, err, "SetExPeriod with period=0 should return error")
	tassert.Contains(t, err.Error(), "period must be positive", "error should mention positive period")
}

func TestNewRedisClient_HSetGet(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	object := &TestObject{Age: 1, Name: "testObject"}

	trequire.NoError(t, client.HSet(ctx, "ObjectH", "key", object), "HSet with key-value pair")

	trequire.NoError(t, client.HSet(ctx, "ObjectH", map[string]interface{}{"key1": object}), "HSet with map[string]interface{}")

	trequire.NoError(t, client.HSet(ctx, "ObjectH", map[string]string{"key2": "key2string"}), "HSet with map[string]string")

	got := &TestObject{}
	trequire.NoError(t, client.HGet(ctx, "ObjectH", "key", got), "HGet key")
	tassert.True(t, cmp.Equal(object, got), "HGet key should match")

	trequire.NoError(t, client.HGet(ctx, "ObjectH", "key1", got), "HGet key1")
	tassert.True(t, cmp.Equal(object, got), "HGet key1 should match")

	strVal, err := client.HGetString(ctx, "ObjectH", "key2")
	trequire.NoError(t, err, "HGetString key2")
	tassert.Equal(t, "key2string", strVal)

	trequire.NoError(t, client.Del(ctx, "ObjectH"), "Del hash key")
}

func TestNewRedisClient_MSetGet(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	object := &TestObject{Age: 1, Name: "testObject"}

	m := map[string]interface{}{
		"mSetKey1": "MSetValue1",
		"mSetKey2": object,
	}
	trequire.NoError(t, client.MSet(ctx, m), "MSet with map[string]interface{}")

	strVal, err := client.GetString(ctx, "mSetKey1")
	trequire.NoError(t, err, "GetString mSetKey1")
	tassert.Equal(t, "MSetValue1", strVal)

	got := &TestObject{}
	trequire.NoError(t, client.Get(ctx, "mSetKey2", got), "Get mSetKey2")
	tassert.True(t, cmp.Equal(object, got), "MSet round-tripped struct should match")

	trequire.NoError(t, client.Del(ctx, "mSetKey1", "mSetKey2"), "Del mset keys")

	m1 := map[string]string{
		"mSetKey1": "MSetValue1",
		"mSetKey2": "MSetValue2",
	}
	trequire.NoError(t, client.MSet(ctx, m1), "MSet with map[string]string")

	strVal, err = client.GetString(ctx, "mSetKey1")
	trequire.NoError(t, err, "GetString mSetKey1 (string map)")
	tassert.Equal(t, "MSetValue1", strVal)

	trequire.NoError(t, client.Del(ctx, "mSetKey1", "mSetKey2"), "Del mset keys (string map)")
}

func TestNewRedisClient_Exist(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "exist1", "v1"), "Set exist1")
	trequire.NoError(t, client.Set(ctx, "exist2", "v2"), "Set exist2")

	exists, err := client.Exist(ctx, "exist1")
	trequire.NoError(t, err, "Exist single key")
	tassert.True(t, exists, "existing key should return true")

	exists, err = client.Exist(ctx, "nonexistent")
	trequire.NoError(t, err, "Exist nonexistent key")
	tassert.False(t, exists, "nonexistent key should return false")

	exists, err = client.Exist(ctx, "exist1", "exist2")
	trequire.NoError(t, err, "Exist multiple keys")
	tassert.True(t, exists, "both existing keys should return true")

	exists, err = client.Exist(ctx, "exist1", "nonexistent")
	trequire.NoError(t, err, "Exist mixed keys")
	tassert.True(t, exists, "one existing key out of two should return true")

	trequire.NoError(t, client.Del(ctx, "exist1", "exist2"), "Del exist keys")
}

func TestNewRedisClient_Expire(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "expirekey", "value"), "Set expirekey")

	ok, err := client.Expire(ctx, "expirekey", time.Second*60)
	trequire.NoError(t, err, "Expire should succeed")
	tassert.True(t, ok, "Expire on existing key should return true")

	trequire.NoError(t, client.Del(ctx, "expirekey"), "Del expirekey")
}

func TestNewRedisClient_HGetAll(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.HSet(ctx, "hashAll", map[string]string{"f1": "v1", "f2": "v2"}), "HSet for HGetAll")

	result, err := client.HGetAll(ctx, "hashAll")
	trequire.NoError(t, err, "HGetAll should succeed")
	tassert.Equal(t, "v1", result["f1"])
	tassert.Equal(t, "v2", result["f2"])

	trequire.NoError(t, client.Del(ctx, "hashAll"), "Del hashAll")
}

func TestNewRedisClient_HDel(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.HSet(ctx, "hashDel", "field1", "value1"), "HSet for HDel")

	trequire.NoError(t, client.HDel(ctx, "hashDel", "field1"), "HDel should succeed")

	_, err := client.HGetString(ctx, "hashDel", "field1")
	tassert.Error(t, err, "HGetString after HDel should fail")
	tassert.True(t, errors.Is(err, ErrNil), "error should be ErrNil, got: %v", err)
}

func TestNewRedisClient_WithKeyPrefix(t *testing.T) {
	client := setupTestClient(t, withKeyPrefix("myapp"))
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "prefixed", "value"), "Set with prefix")

	tassert.Equal(t, "myapp:prefixed", client.GetKey("prefixed"), "GetKey should include separator")

	got, err := client.GetString(ctx, "prefixed")
	trequire.NoError(t, err, "GetString with prefix")
	tassert.Equal(t, "value", got)

	trequire.NoError(t, client.Del(ctx, "prefixed"), "Del with prefix")
}

func TestNewRedisClient_WithMetrics(t *testing.T) {
	client := setupTestClient(t, withMetrics())
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "metrickey", "metricval"), "Set with metrics enabled")

	got, err := client.GetString(ctx, "metrickey")
	trequire.NoError(t, err, "GetString with metrics")
	tassert.Equal(t, "metricval", got)

	trequire.NoError(t, client.Del(ctx, "metrickey"), "Del with metrics")
}

func TestNewRedisClient_EmptyAddrs(t *testing.T) {
	_, err := NewRedisClient(Config{
		Addrs: []string{},
		DB:    0,
	})
	tassert.Error(t, err, "empty Addrs should return error")
	tassert.Contains(t, err.Error(), "addrs must not be empty", "error message should mention addrs")
}

func TestNewRedisClient_EncodingObjectNil(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "nilkey", nil), "Set nil value should not panic")

	// encodingObject(nil) returns nil, which redis.Set serializes as empty string ""
	got, err := client.GetString(ctx, "nilkey")
	trequire.NoError(t, err, "GetString nilkey")
	tassert.Equal(t, "", got, "nil value should be stored as empty string")

	trequire.NoError(t, client.Del(ctx, "nilkey"), "Del nilkey")
}

func TestNewRedisClient_MSetEmpty(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.MSet(ctx)
	tassert.Error(t, err, "MSet with no values should return error")
}

func TestNewRedisClient_GetMissingKey_ReturnsErrNil(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.Get(ctx, "nonexistent", &TestObject{})
	tassert.Error(t, err, "Get missing key should return error")
	tassert.True(t, errors.Is(err, ErrNil), "error should be ErrNil, got: %v", err)
}

func TestNewRedisClient_GetStringMissingKey_ReturnsErrNil(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	_, err := client.GetString(ctx, "nonexistent")
	tassert.Error(t, err, "GetString missing key should return error")
	tassert.True(t, errors.Is(err, ErrNil), "error should be ErrNil, got: %v", err)
}

func TestNewRedisClient_HGetMissingField_ReturnsErrNil(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.HSet(ctx, "hashkey", "field1", "value1"), "HSet")

	err := client.HGet(ctx, "hashkey", "nonexistent", &TestObject{})
	tassert.Error(t, err, "HGet missing field should return error")
	tassert.True(t, errors.Is(err, ErrNil), "error should be ErrNil, got: %v", err)
}

func TestNewRedisClient_MSetNX(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	m := map[string]string{"msetnxkey": "value1"}
	trequire.NoError(t, client.MSetNX(ctx, m), "MSetNX should succeed")

	got, err := client.GetString(ctx, "msetnxkey")
	trequire.NoError(t, err, "GetString msetnxkey")
	tassert.Equal(t, "value1", got)

	trequire.NoError(t, client.Del(ctx, "msetnxkey"), "Del msetnxkey")
}

func TestMaskPassword(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"ab", "****"},
		{"abcd", "****"},
		{"abcdef", "ab**ef"},
		{"mypassword123", "my*********23"},
		{"密码abc", "密码*bc"},
	}
	for _, tt := range tests {
		got := maskPassword(tt.input)
		tassert.Equal(t, tt.expected, got, "maskPassword(%q)", tt.input)
	}
}

func TestNewRedisClient_CloseIdempotent(t *testing.T) {
	client := setupTestClient(t)

	rc, ok := client.(*redisClient)
	trequire.True(t, ok, "should be *redisClient")
	rc.Close()
	rc.Close()
	rc.Close()

	err := rc.Set(context.Background(), "after_close", "val")
	tassert.Error(t, err, "operation after Close should fail")
}

// --- Edge case / boundary condition tests ---

func TestEncodingObject_Map(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := map[string]int{"a": 1, "b": 2}
	trequire.NoError(t, client.Set(ctx, "mapkey", original), "Set map value")

	var got map[string]int
	trequire.NoError(t, client.Get(ctx, "mapkey", &got), "Get map value")
	tassert.Equal(t, original, got, "round-tripped map should match")

	trequire.NoError(t, client.Del(ctx, "mapkey"), "Del mapkey")
}

func TestEncodingObject_Slice(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := []string{"x", "y", "z"}
	trequire.NoError(t, client.Set(ctx, "slicekey", original), "Set slice value")

	var got []string
	trequire.NoError(t, client.Get(ctx, "slicekey", &got), "Get slice value")
	tassert.Equal(t, original, got, "round-tripped slice should match")

	trequire.NoError(t, client.Del(ctx, "slicekey"), "Del slicekey")
}

func TestEncodingObject_IntSlice(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := []int{10, 20, 30}
	trequire.NoError(t, client.Set(ctx, "intslicekey", original), "Set int slice value")

	var got []int
	trequire.NoError(t, client.Get(ctx, "intslicekey", &got), "Get int slice value")
	tassert.Equal(t, original, got, "round-tripped int slice should match")

	trequire.NoError(t, client.Del(ctx, "intslicekey"), "Del intslicekey")
}

func TestEncodingObject_EmptySlice(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := []string{}
	trequire.NoError(t, client.Set(ctx, "emptyslicekey", original), "Set empty slice value")

	var got []string
	trequire.NoError(t, client.Get(ctx, "emptyslicekey", &got), "Get empty slice value")
	tassert.Empty(t, got, "empty slice should round-trip")

	trequire.NoError(t, client.Del(ctx, "emptyslicekey"), "Del emptyslicekey")
}

func TestEncodingObject_EmptyMap(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := map[string]string{}
	trequire.NoError(t, client.Set(ctx, "emptymapkey", original), "Set empty map value")

	var got map[string]string
	trequire.NoError(t, client.Get(ctx, "emptymapkey", &got), "Get empty map value")
	tassert.Empty(t, got, "empty map should round-trip")

	trequire.NoError(t, client.Del(ctx, "emptymapkey"), "Del emptymapkey")
}

func TestEncodingValues_OddValues(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.MSet(ctx, "key1", "val1", "key2")
	tassert.Error(t, err, "MSet with odd number of key-value pairs should return error")
	tassert.Contains(t, err.Error(), "must be even", "error should mention even requirement")
}

func TestDel_EmptyArgs(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.Del(ctx)
	tassert.NoError(t, err, "Del with no keys should return nil without error")
}

func TestExist_EmptyArgs(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	exists, err := client.Exist(ctx)
	tassert.NoError(t, err, "Exist with no keys should return nil without error")
	tassert.False(t, exists, "Exist with no keys should return false")
}

func TestHGetAll_NonexistentKey(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	result, err := client.HGetAll(ctx, "nonexistent_hash")
	tassert.NoError(t, err, "HGetAll on nonexistent key should not return error")
	tassert.Empty(t, result, "HGetAll on nonexistent key should return empty map")
}

func TestSetExPeriod_NegativePeriod(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	err := client.SetExPeriod(ctx, "negkey", "val", -1*time.Second)
	tassert.Error(t, err, "SetExPeriod with negative period should return error")
	tassert.Contains(t, err.Error(), "period must be positive", "error should mention positive period")
}

func TestSetNXPeriod(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	ok, err := client.SetNXPeriod(ctx, "nxperiodkey", "value1", 60*time.Second)
	trequire.NoError(t, err, "SetNXPeriod first time should not return error")
	tassert.True(t, ok, "SetNXPeriod first time should return true")

	ok, err = client.SetNXPeriod(ctx, "nxperiodkey", "value2", 60*time.Second)
	trequire.NoError(t, err, "SetNXPeriod on existing key should not return error")
	tassert.False(t, ok, "SetNXPeriod on existing key should return false")

	got, err := client.GetString(ctx, "nxperiodkey")
	trequire.NoError(t, err, "GetString nxperiodkey")
	tassert.Equal(t, "value1", got, "value should remain as value1")
}

func TestMSet_MapWithStructAndSlice(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 42, Name: "alice"}
	sl := []int{1, 2, 3}

	m := map[string]interface{}{
		"structkey": obj,
		"slicekey":  sl,
	}
	trequire.NoError(t, client.MSet(ctx, m), "MSet with struct and slice values")

	var gotObj TestObject
	trequire.NoError(t, client.Get(ctx, "structkey", &gotObj), "Get structkey")
	tassert.Equal(t, *obj, gotObj, "struct should round-trip via MSet")

	var gotSlice []int
	trequire.NoError(t, client.Get(ctx, "slicekey", &gotSlice), "Get slicekey")
	tassert.Equal(t, sl, gotSlice, "slice should round-trip via MSet")

	trequire.NoError(t, client.Del(ctx, "structkey", "slicekey"), "Del keys")
}

func TestHSet_StructValue(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 99, Name: "bob"}
	trequire.NoError(t, client.HSet(ctx, "hstruct", "field1", obj), "HSet struct value")

	var got TestObject
	trequire.NoError(t, client.HGet(ctx, "hstruct", "field1", &got), "HGet struct value")
	tassert.Equal(t, *obj, got, "struct should round-trip via HSet/HGet")

	trequire.NoError(t, client.Del(ctx, "hstruct"), "Del hstruct")
}

func TestNewRedisClient_EmptyAddrs_Nil(t *testing.T) {
	_, err := NewRedisClient(Config{
		Addrs: nil,
		DB:    0,
	})
	tassert.Error(t, err, "nil Addrs should return error")
	tassert.Contains(t, err.Error(), "addrs must not be empty", "error message should mention addrs")
}

func TestNewRedisClient_EncodingObject_Primitives(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "intkey", 42), "Set int value")
	got, err := client.GetString(ctx, "intkey")
	trequire.NoError(t, err, "GetString intkey")
	tassert.Equal(t, "42", got, "int should be stored as string representation")

	trequire.NoError(t, client.Set(ctx, "floatkey", 3.14), "Set float value")
	got, err = client.GetString(ctx, "floatkey")
	trequire.NoError(t, err, "GetString floatkey")
	tassert.Equal(t, "3.14", got, "float should be stored as string representation")

	trequire.NoError(t, client.Set(ctx, "boolkey", true), "Set bool value")
	got, err = client.GetString(ctx, "boolkey")
	trequire.NoError(t, err, "GetString boolkey")
	tassert.Equal(t, "1", got, "bool true should be stored as '1' by go-redis")

	trequire.NoError(t, client.Del(ctx, "intkey", "floatkey", "boolkey"), "Del primitive keys")
}

func TestEncodingObject_ByteSlice(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := []byte{0x01, 0x02, 0x03, 0xff}
	trequire.NoError(t, client.Set(ctx, "byteslicekey", original), "Set []byte value")

	got, err := client.GetString(ctx, "byteslicekey")
	trequire.NoError(t, err, "GetString byteslicekey")
	tassert.Equal(t, string(original), got, "[]byte should be stored as raw binary string, not JSON array")

	trequire.NoError(t, client.Del(ctx, "byteslicekey"), "Del byteslicekey")
}

func TestEncodingObject_ByteSliceEmpty(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	original := []byte{}
	trequire.NoError(t, client.Set(ctx, "emptybyteslicekey", original), "Set empty []byte value")

	got, err := client.GetString(ctx, "emptybyteslicekey")
	trequire.NoError(t, err, "GetString emptybyteslicekey")
	tassert.Equal(t, "", got, "empty []byte should be stored as empty string")

	trequire.NoError(t, client.Del(ctx, "emptybyteslicekey"), "Del emptybyteslicekey")
}

// --- Counter tests ---

func TestIncr(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	val, err := client.Incr(ctx, "counter")
	trequire.NoError(t, err, "Incr first time")
	tassert.Equal(t, int64(1), val, "Incr from 0 should return 1")

	val, err = client.Incr(ctx, "counter")
	trequire.NoError(t, err, "Incr second time")
	tassert.Equal(t, int64(2), val, "Incr from 1 should return 2")

	trequire.NoError(t, client.Del(ctx, "counter"), "Del counter")
}

func TestIncrBy(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	val, err := client.IncrBy(ctx, "counter", 10)
	trequire.NoError(t, err, "IncrBy 10")
	tassert.Equal(t, int64(10), val, "IncrBy 10 from 0 should return 10")

	val, err = client.IncrBy(ctx, "counter", -3)
	trequire.NoError(t, err, "IncrBy -3")
	tassert.Equal(t, int64(7), val, "IncrBy -3 from 10 should return 7")

	trequire.NoError(t, client.Del(ctx, "counter"), "Del counter")
}

func TestDecr(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "counter", "5"), "Set counter to 5")

	val, err := client.Decr(ctx, "counter")
	trequire.NoError(t, err, "Decr")
	tassert.Equal(t, int64(4), val, "Decr from 5 should return 4")

	trequire.NoError(t, client.Del(ctx, "counter"), "Del counter")
}

func TestDecrBy(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.Set(ctx, "counter", "10"), "Set counter to 10")

	val, err := client.DecrBy(ctx, "counter", 3)
	trequire.NoError(t, err, "DecrBy 3")
	tassert.Equal(t, int64(7), val, "DecrBy 3 from 10 should return 7")

	trequire.NoError(t, client.Del(ctx, "counter"), "Del counter")
}

// --- TTL test ---

func TestTTL(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.SetExPeriod(ctx, "ttlkey", "val", 60*time.Second), "SetExPeriod 60s")

	ttl, err := client.TTL(ctx, "ttlkey")
	trequire.NoError(t, err, "TTL on existing key")
	tassert.True(t, ttl > 0 && ttl <= 60*time.Second, "TTL should be between 0 and 60s, got %v", ttl)

	ttl, err = client.TTL(ctx, "nonexistent_ttl_key")
	trequire.NoError(t, err, "TTL on nonexistent key")
	tassert.Equal(t, time.Duration(-2), ttl, "TTL on nonexistent key should return -2")

	trequire.NoError(t, client.Del(ctx, "ttlkey"), "Del ttlkey")
}

// --- List tests ---

func TestLPush_RPush_LRange(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.RPush(ctx, "listkey", "a", "b", "c"), "RPush a b c")
	trequire.NoError(t, client.LPush(ctx, "listkey", "z"), "LPush z")

	vals, err := client.LRange(ctx, "listkey", 0, -1)
	trequire.NoError(t, err, "LRange")
	tassert.Equal(t, []string{"z", "a", "b", "c"}, vals, "list should be [z a b c]")

	length, err := client.LLen(ctx, "listkey")
	trequire.NoError(t, err, "LLen")
	tassert.Equal(t, int64(4), length, "LLen should be 4")

	trequire.NoError(t, client.Del(ctx, "listkey"), "Del listkey")
}

func TestLPop_RPop(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.RPush(ctx, "poplist", "a", "b", "c"), "RPush a b c")

	left, err := client.LPop(ctx, "poplist")
	trequire.NoError(t, err, "LPop")
	tassert.Equal(t, "a", left, "LPop should return a")

	right, err := client.RPop(ctx, "poplist")
	trequire.NoError(t, err, "RPop")
	tassert.Equal(t, "c", right, "RPop should return c")

	vals, err := client.LRange(ctx, "poplist", 0, -1)
	trequire.NoError(t, err, "LRange after pops")
	tassert.Equal(t, []string{"b"}, vals, "remaining list should be [b]")

	_, err = client.LPop(ctx, "never_created_list")
	tassert.True(t, errors.Is(err, ErrNil), "LPop on never-created list should return ErrNil")

	trequire.NoError(t, client.Del(ctx, "poplist"), "Del poplist")
}

// --- Set tests ---

func TestSAdd_SMembers_SCard(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.SAdd(ctx, "setkey", "a", "b", "c"), "SAdd a b c")

	card, err := client.SCard(ctx, "setkey")
	trequire.NoError(t, err, "SCard")
	tassert.Equal(t, int64(3), card, "SCard should be 3")

	members, err := client.SMembers(ctx, "setkey")
	trequire.NoError(t, err, "SMembers")
	tassert.Len(t, members, 3, "SMembers should have 3 elements")

	isMember, err := client.SIsMember(ctx, "setkey", "a")
	trequire.NoError(t, err, "SIsMember a")
	tassert.True(t, isMember, "a should be a member")

	isMember, err = client.SIsMember(ctx, "setkey", "z")
	trequire.NoError(t, err, "SIsMember z")
	tassert.False(t, isMember, "z should not be a member")

	trequire.NoError(t, client.Del(ctx, "setkey"), "Del setkey")
}

func TestSRem(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.SAdd(ctx, "setkey2", "a", "b", "c"), "SAdd a b c")
	trequire.NoError(t, client.SRem(ctx, "setkey2", "b"), "SRem b")

	members, err := client.SMembers(ctx, "setkey2")
	trequire.NoError(t, err, "SMembers after SRem")
	tassert.Len(t, members, 2, "SMembers should have 2 elements after SRem")

	trequire.NoError(t, client.Del(ctx, "setkey2"), "Del setkey2")
}

// --- Sorted Set tests ---

func TestZAdd_ZRange_ZCard(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.ZAdd(ctx, "zsetkey",
		&redis.Z{Score: 1, Member: "a"},
		&redis.Z{Score: 2, Member: "b"},
		&redis.Z{Score: 3, Member: "c"},
	), "ZAdd a b c")

	card, err := client.ZCard(ctx, "zsetkey")
	trequire.NoError(t, err, "ZCard")
	tassert.Equal(t, int64(3), card, "ZCard should be 3")

	vals, err := client.ZRange(ctx, "zsetkey", 0, -1)
	trequire.NoError(t, err, "ZRange")
	tassert.Equal(t, []string{"a", "b", "c"}, vals, "ZRange should return [a b c] in score order")

	scores, err := client.ZRangeWithScores(ctx, "zsetkey", 0, -1)
	trequire.NoError(t, err, "ZRangeWithScores")
	trequire.Len(t, scores, 3, "ZRangeWithScores should return 3 elements")
	tassert.Equal(t, float64(1), scores[0].Score, "first element score should be 1")
	tassert.Equal(t, "a", scores[0].Member, "first element member should be a")

	trequire.NoError(t, client.Del(ctx, "zsetkey"), "Del zsetkey")
}

func TestZRevRange(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.ZAdd(ctx, "zsetrev",
		&redis.Z{Score: 1, Member: "a"},
		&redis.Z{Score: 2, Member: "b"},
		&redis.Z{Score: 3, Member: "c"},
	), "ZAdd")

	vals, err := client.ZRevRange(ctx, "zsetrev", 0, -1)
	trequire.NoError(t, err, "ZRevRange")
	tassert.Equal(t, []string{"c", "b", "a"}, vals, "ZRevRange should return [c b a]")

	scores, err := client.ZRevRangeWithScores(ctx, "zsetrev", 0, -1)
	trequire.NoError(t, err, "ZRevRangeWithScores")
	trequire.Len(t, scores, 3, "ZRevRangeWithScores should return 3 elements")
	tassert.Equal(t, float64(3), scores[0].Score, "first element score should be 3")

	trequire.NoError(t, client.Del(ctx, "zsetrev"), "Del zsetrev")
}

func TestZScore_ZRank(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.ZAdd(ctx, "zsetscore",
		&redis.Z{Score: 10, Member: "x"},
		&redis.Z{Score: 20, Member: "y"},
	), "ZAdd")

	score, err := client.ZScore(ctx, "zsetscore", "x")
	trequire.NoError(t, err, "ZScore x")
	tassert.Equal(t, float64(10), score, "ZScore of x should be 10")

	rank, err := client.ZRank(ctx, "zsetscore", "y")
	trequire.NoError(t, err, "ZRank y")
	tassert.Equal(t, int64(1), rank, "ZRank of y should be 1")

	revRank, err := client.ZRevRank(ctx, "zsetscore", "x")
	trequire.NoError(t, err, "ZRevRank x")
	tassert.Equal(t, int64(1), revRank, "ZRevRank of x should be 1")

	_, err = client.ZScore(ctx, "zsetscore", "nonexistent")
	tassert.True(t, errors.Is(err, ErrNil), "ZScore on nonexistent member should return ErrNil")

	trequire.NoError(t, client.Del(ctx, "zsetscore"), "Del zsetscore")
}

func TestZRem(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	trequire.NoError(t, client.ZAdd(ctx, "zsetrem",
		&redis.Z{Score: 1, Member: "a"},
		&redis.Z{Score: 2, Member: "b"},
	), "ZAdd")

	trequire.NoError(t, client.ZRem(ctx, "zsetrem", "a"), "ZRem a")

	card, err := client.ZCard(ctx, "zsetrem")
	trequire.NoError(t, err, "ZCard after ZRem")
	tassert.Equal(t, int64(1), card, "ZCard should be 1 after ZRem")

	trequire.NoError(t, client.Del(ctx, "zsetrem"), "Del zsetrem")
}

func TestLPush_StructValue(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 42, Name: "alice"}
	trequire.NoError(t, client.LPush(ctx, "liststruct", obj), "LPush struct value")

	vals, err := client.LRange(ctx, "liststruct", 0, -1)
	trequire.NoError(t, err, "LRange liststruct")
	trequire.Len(t, vals, 1, "list should have 1 element")

	var got TestObject
	trequire.NoError(t, tools.Unmarshal([]byte(vals[0]), &got), "Unmarshal list element")
	tassert.Equal(t, *obj, got, "struct should round-trip via LPush/LRange")

	trequire.NoError(t, client.Del(ctx, "liststruct"), "Del liststruct")
}

func TestRPush_SliceValue(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	sl := []int{1, 2, 3}
	trequire.NoError(t, client.RPush(ctx, "listslice", sl), "RPush slice value")

	vals, err := client.LRange(ctx, "listslice", 0, -1)
	trequire.NoError(t, err, "LRange listslice")
	trequire.Len(t, vals, 1, "list should have 1 element")

	var got []int
	trequire.NoError(t, tools.Unmarshal([]byte(vals[0]), &got), "Unmarshal list element")
	tassert.Equal(t, sl, got, "slice should round-trip via RPush/LRange")

	trequire.NoError(t, client.Del(ctx, "listslice"), "Del listslice")
}

func TestSAdd_StructValue(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 30, Name: "bob"}
	trequire.NoError(t, client.SAdd(ctx, "setstruct", obj), "SAdd struct value")

	members, err := client.SMembers(ctx, "setstruct")
	trequire.NoError(t, err, "SMembers setstruct")
	trequire.Len(t, members, 1, "set should have 1 element")

	var got TestObject
	trequire.NoError(t, tools.Unmarshal([]byte(members[0]), &got), "Unmarshal set member")
	tassert.Equal(t, *obj, got, "struct should round-trip via SAdd/SMembers")

	trequire.NoError(t, client.Del(ctx, "setstruct"), "Del setstruct")
}

func TestSRem_StructValue(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj1 := &TestObject{Age: 1, Name: "a"}
	obj2 := &TestObject{Age: 2, Name: "b"}
	trequire.NoError(t, client.SAdd(ctx, "setremstruct", obj1, obj2), "SAdd two structs")

	trequire.NoError(t, client.SRem(ctx, "setremstruct", obj1), "SRem struct value")

	card, err := client.SCard(ctx, "setremstruct")
	trequire.NoError(t, err, "SCard after SRem")
	tassert.Equal(t, int64(1), card, "SCard should be 1 after SRem")

	trequire.NoError(t, client.Del(ctx, "setremstruct"), "Del setremstruct")
}

func TestSIsMember_StructValue(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 42, Name: "charlie"}
	trequire.NoError(t, client.SAdd(ctx, "setismember", obj), "SAdd struct value")

	isMember, err := client.SIsMember(ctx, "setismember", obj)
	trequire.NoError(t, err, "SIsMember struct value")
	tassert.True(t, isMember, "struct added via SAdd should be found via SIsMember")

	notObj := &TestObject{Age: 99, Name: "nobody"}
	isMember, err = client.SIsMember(ctx, "setismember", notObj)
	trequire.NoError(t, err, "SIsMember nonexistent struct")
	tassert.False(t, isMember, "nonexistent struct should not be a member")

	trequire.NoError(t, client.Del(ctx, "setismember"), "Del setismember")
}

func TestZAdd_ZRem_ManualSerialization(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 25, Name: "dave"}
	jsonBytes, err := tools.Marshal(obj)
	trequire.NoError(t, err, "Marshal struct")
	jsonStr := string(jsonBytes)

	trequire.NoError(t, client.ZAdd(ctx, "zsetmanual",
		&redis.Z{Score: 1, Member: jsonStr},
	), "ZAdd with manually serialized member")

	vals, err := client.ZRange(ctx, "zsetmanual", 0, -1)
	trequire.NoError(t, err, "ZRange zsetmanual")
	trequire.Len(t, vals, 1, "zset should have 1 element")
	tassert.Equal(t, jsonStr, vals[0], "ZRange should return the JSON string")

	trequire.NoError(t, client.ZRem(ctx, "zsetmanual", jsonStr), "ZRem with same serialized value")

	card, err := client.ZCard(ctx, "zsetmanual")
	trequire.NoError(t, err, "ZCard after ZRem")
	tassert.Equal(t, int64(0), card, "ZCard should be 0 after ZRem")

	trequire.NoError(t, client.Del(ctx, "zsetmanual"), "Del zsetmanual")
}

// --- Struct round-trip tests for all commands that support encodingObject ---

func TestSetPeriod_StructRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 30, Name: "period_struct"}
	trequire.NoError(t, client.SetPeriod(ctx, "period_struct", obj, 60*time.Second), "SetPeriod struct")

	var got TestObject
	trequire.NoError(t, client.Get(ctx, "period_struct", &got), "Get period_struct")
	tassert.Equal(t, *obj, got, "struct should round-trip via SetPeriod/Get")

	trequire.NoError(t, client.Del(ctx, "period_struct"), "Del period_struct")
}

func TestSetNX_StructRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 25, Name: "nx_struct"}
	ok, err := client.SetNX(ctx, "nx_struct", obj)
	trequire.NoError(t, err, "SetNX struct")
	tassert.True(t, ok, "SetNX should succeed on new key")

	var got TestObject
	trequire.NoError(t, client.Get(ctx, "nx_struct", &got), "Get nx_struct")
	tassert.Equal(t, *obj, got, "struct should round-trip via SetNX/Get")

	obj2 := &TestObject{Age: 99, Name: "nx_struct_2"}
	ok, err = client.SetNX(ctx, "nx_struct", obj2)
	trequire.NoError(t, err, "SetNX on existing key")
	tassert.False(t, ok, "SetNX should return false on existing key")

	trequire.NoError(t, client.Get(ctx, "nx_struct", &got), "Get nx_struct after second SetNX")
	tassert.Equal(t, *obj, got, "value should remain unchanged after failed SetNX")

	trequire.NoError(t, client.Del(ctx, "nx_struct"), "Del nx_struct")
}

func TestSetNXPeriod_StructRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 40, Name: "nxperiod_struct"}
	ok, err := client.SetNXPeriod(ctx, "nxperiod_struct", obj, 60*time.Second)
	trequire.NoError(t, err, "SetNXPeriod struct")
	tassert.True(t, ok, "SetNXPeriod should succeed on new key")

	var got TestObject
	trequire.NoError(t, client.Get(ctx, "nxperiod_struct", &got), "Get nxperiod_struct")
	tassert.Equal(t, *obj, got, "struct should round-trip via SetNXPeriod/Get")

	trequire.NoError(t, client.Del(ctx, "nxperiod_struct"), "Del nxperiod_struct")
}

func TestSetExPeriod_StructRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 50, Name: "setex_struct"}
	trequire.NoError(t, client.SetExPeriod(ctx, "setex_struct", obj, 60*time.Second), "SetExPeriod struct")

	var got TestObject
	trequire.NoError(t, client.Get(ctx, "setex_struct", &got), "Get setex_struct")
	tassert.Equal(t, *obj, got, "struct should round-trip via SetExPeriod/Get")

	trequire.NoError(t, client.Del(ctx, "setex_struct"), "Del setex_struct")
}

func TestMSetNX_StructRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 35, Name: "msetnx_struct"}
	sl := []string{"x", "y", "z"}

	m := map[string]interface{}{
		"msetnx_struct_key": obj,
		"msetnx_slice_key":  sl,
	}
	trequire.NoError(t, client.MSetNX(ctx, m), "MSetNX with struct and slice")

	var gotObj TestObject
	trequire.NoError(t, client.Get(ctx, "msetnx_struct_key", &gotObj), "Get msetnx_struct_key")
	tassert.Equal(t, *obj, gotObj, "struct should round-trip via MSetNX/Get")

	var gotSlice []string
	trequire.NoError(t, client.Get(ctx, "msetnx_slice_key", &gotSlice), "Get msetnx_slice_key")
	tassert.Equal(t, sl, gotSlice, "slice should round-trip via MSetNX/Get")

	trequire.NoError(t, client.Del(ctx, "msetnx_struct_key", "msetnx_slice_key"), "Del keys")
}

func TestHSet_MapStructRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj1 := &TestObject{Age: 10, Name: "field1_obj"}
	obj2 := &TestObject{Age: 20, Name: "field2_obj"}

	trequire.NoError(t, client.HSet(ctx, "hmap_struct", map[string]interface{}{
		"field1": obj1,
		"field2": obj2,
	}), "HSet with map[string]interface{} containing structs")

	var got1 TestObject
	trequire.NoError(t, client.HGet(ctx, "hmap_struct", "field1", &got1), "HGet field1")
	tassert.Equal(t, *obj1, got1, "field1 struct should round-trip")

	var got2 TestObject
	trequire.NoError(t, client.HGet(ctx, "hmap_struct", "field2", &got2), "HGet field2")
	tassert.Equal(t, *obj2, got2, "field2 struct should round-trip")

	trequire.NoError(t, client.Del(ctx, "hmap_struct"), "Del hmap_struct")
}

func TestHGetString_StructValue(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	obj := &TestObject{Age: 55, Name: "hgetstr_struct"}
	trequire.NoError(t, client.HSet(ctx, "hgetstr_hash", "myfield", obj), "HSet struct value")

	strVal, err := client.HGetString(ctx, "hgetstr_hash", "myfield")
	trequire.NoError(t, err, "HGetString struct field")
	tassert.NotEmpty(t, strVal, "HGetString should return non-empty JSON string")

	var got TestObject
	trequire.NoError(t, tools.Unmarshal([]byte(strVal), &got), "Unmarshal HGetString result")
	tassert.Equal(t, *obj, got, "struct should round-trip via HSet/HGetString+Unmarshal")

	trequire.NoError(t, client.Del(ctx, "hgetstr_hash"), "Del hgetstr_hash")
}

func TestSet_MapRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	m := map[string]int{"a": 1, "b": 2}
	trequire.NoError(t, client.Set(ctx, "map_key", m), "Set map value")

	var got map[string]int
	trequire.NoError(t, client.Get(ctx, "map_key", &got), "Get map_key")
	tassert.Equal(t, m, got, "map should round-trip via Set/Get")

	trequire.NoError(t, client.Del(ctx, "map_key"), "Del map_key")
}

func TestSet_SliceRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	sl := []string{"hello", "world"}
	trequire.NoError(t, client.Set(ctx, "slice_key", sl), "Set slice value")

	var got []string
	trequire.NoError(t, client.Get(ctx, "slice_key", &got), "Get slice_key")
	tassert.Equal(t, sl, got, "slice should round-trip via Set/Get")

	trequire.NoError(t, client.Del(ctx, "slice_key"), "Del slice_key")
}

func TestSet_ArrayRoundTrip(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	arr := [3]int{10, 20, 30}
	trequire.NoError(t, client.Set(ctx, "array_key", arr), "Set array value")

	var got [3]int
	trequire.NoError(t, client.Get(ctx, "array_key", &got), "Get array_key")
	tassert.Equal(t, arr, got, "array should round-trip via Set/Get")

	trequire.NoError(t, client.Del(ctx, "array_key"), "Del array_key")
}

func TestEncodingObject_ByteArray(t *testing.T) {
	client := setupTestClient(t)
	ctx := context.Background()

	arr := [4]byte{0x01, 0x02, 0x03, 0xff}
	trequire.NoError(t, client.Set(ctx, "bytearray_key", arr), "Set [4]byte value")

	strVal, err := client.GetString(ctx, "bytearray_key")
	trequire.NoError(t, err, "GetString bytearray_key")
	tassert.NotEqual(t, "", strVal, "[4]byte should be JSON-encoded, not Go format")

	var got [4]byte
	trequire.NoError(t, tools.Unmarshal([]byte(strVal), &got), "Unmarshal bytearray")
	tassert.Equal(t, arr, got, "[4]byte should round-trip via JSON encoding")

	trequire.NoError(t, client.Del(ctx, "bytearray_key"), "Del bytearray_key")
}
