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
