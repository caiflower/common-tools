package redisd

import (
	"encoding/json"
	"fmt"
	"strconv"

	v2 "github.com/caiflower/common-tools/redis/v2"
)

// Default key prefix for all taskx Redis keys.
const DefaultKeyPrefix = "taskx"

// KeyConfig holds configurable key prefix for Redis storage.
type KeyConfig struct {
	Prefix string `yaml:"prefix" json:"prefix"`
}

func DefaultKeyConfig() *KeyConfig {
	return &KeyConfig{Prefix: DefaultKeyPrefix}
}

func (c *KeyConfig) Normalize() *KeyConfig {
	if c == nil || c.Prefix == "" {
		return DefaultKeyConfig()
	}
	return c
}

// keyBuilder constructs Redis keys with the configured prefix.
type keyBuilder struct {
	prefix string
}

func newKeyBuilder(cfg *KeyConfig) *keyBuilder {
	return &keyBuilder{prefix: cfg.Normalize().Prefix}
}

// --- Entity keys (Hash) ---

func (k *keyBuilder) taskKey(id string) string {
	return fmt.Sprintf("%s:task:{%s}", k.prefix, id)
}

func (k *keyBuilder) subtaskKey(id string) string {
	return fmt.Sprintf("%s:subtask:{%s}", k.prefix, id)
}

func (k *keyBuilder) edgeKey(id string) string {
	return fmt.Sprintf("%s:edge:{%s}", k.prefix, id)
}

func (k *keyBuilder) bakTaskKey(id string) string {
	return fmt.Sprintf("%s:bak:task:{%s}", k.prefix, id)
}

func (k *keyBuilder) bakSubtaskKey(id string) string {
	return fmt.Sprintf("%s:bak:subtask:{%s}", k.prefix, id)
}

// --- Index keys (Sorted Set / Set) ---

func (k *keyBuilder) todoSetKey() string {
	return k.prefix + ":todo"
}

func (k *keyBuilder) subtaskIndexKey(taskID string) string {
	return fmt.Sprintf("%s:task:{%s}:subtasks", k.prefix, taskID)
}

func (k *keyBuilder) edgeIndexKey(taskID string) string {
	return fmt.Sprintf("%s:task:{%s}:edges", k.prefix, taskID)
}

func (k *keyBuilder) bakSubtaskIndexKey(taskID string) string {
	return fmt.Sprintf("%s:bak:task:{%s}:subtasks", k.prefix, taskID)
}

// --- Serialization helpers ---

// toHash converts a struct to map[string]string suitable for Redis HSET.
// Uses JSON marshal → map → string values pipeline.
func toHash(v interface{}) (map[string]string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("toHash marshal: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("toHash unmarshal raw: %w", err)
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		// Store raw JSON value as string (avoids double-encoding)
		result[k] = string(v)
	}
	return result, nil
}

// fromHash reconstructs a struct from a Redis HGETALL result map.
func fromHash(m map[string]string, v interface{}) error {
	raw := make(map[string]json.RawMessage, len(m))
	for k, val := range m {
		raw[k] = json.RawMessage(val)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("fromHash marshal raw: %w", err)
	}
	return json.Unmarshal(data, v)
}

// parseInt8 parses a string to int8, returning 0 on error.
func parseInt8(s string) int8 {
	n, _ := strconv.ParseInt(s, 10, 8)
	return int8(n)
}

// parseInt parses a string to int, returning 0 on error.
func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// parseInt32 parses a string to int32, returning 0 on error.
func parseInt32(s string) int32 {
	n, _ := strconv.ParseInt(s, 10, 32)
	return int32(n)
}

// parseBool parses a string to bool ("true" = true, else false).
func parseBool(s string) bool {
	return s == "true"
}

// cmd returns the redis Cmdable with key prefix support.
func cmd(client v2.RedisClient) v2.Cmdable {
	return client.Cmd()
}
