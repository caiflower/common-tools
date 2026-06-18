package redisd

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/caiflower/common-tools/pkg/json"
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
// JSON string values are unquoted for natural Redis storage and CAS comparison.
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
		s := string(v)
		// Unquote JSON strings for natural Redis storage (enables direct CAS comparison)
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			var unquoted string
			if err := json.Unmarshal(v, &unquoted); err == nil {
				s = unquoted
			}
		}
		result[k] = s
	}
	return result, nil
}

// fromHash reconstructs a struct from a Redis HGETALL result map.
// Uses struct field types to determine how to interpret Redis string values.
func fromHash(m map[string]string, v interface{}) error {
	// Build JSON tag → field kind map from struct type
	tagMap := buildTagMap(v)

	raw := make(map[string]json.RawMessage, len(m))
	for k, val := range m {
		kind := tagMap[k]
		raw[k] = valueToJSON(val, kind)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("fromHash marshal raw: %w", err)
	}
	return json.Unmarshal(data, v)
}

// buildTagMap returns a map of json tag name → reflect.Kind for the struct type.
func buildTagMap(v interface{}) map[string]reflect.Kind {
	result := make(map[string]reflect.Kind)
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return result
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		// Handle json:"name,omitempty"
		name := tag
		for j := 0; j < len(tag); j++ {
			if tag[j] == ',' {
				name = tag[:j]
				break
			}
		}
		result[name] = f.Type.Kind()
	}
	return result
}

// valueToJSON converts a Redis string value to a json.RawMessage based on the target kind.
func valueToJSON(val string, kind reflect.Kind) json.RawMessage {
	if val == "" {
		// Empty string → null for non-string types, empty string for string
		if kind == reflect.String || kind == reflect.Invalid {
			return json.RawMessage(`""`)
		}
		return json.RawMessage(`null`)
	}

	switch kind {
	case reflect.String:
		// Always quote as JSON string
		quoted, _ := json.Marshal(val)
		return quoted
	case reflect.Bool:
		if val == "true" || val == "false" {
			return json.RawMessage(val)
		}
		return json.RawMessage(`false`)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if _, err := strconv.ParseInt(val, 10, 64); err == nil {
			return json.RawMessage(val)
		}
		return json.RawMessage(`0`)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if _, err := strconv.ParseUint(val, 10, 64); err == nil {
			return json.RawMessage(val)
		}
		return json.RawMessage(`0`)
	case reflect.Float32, reflect.Float64:
		if _, err := strconv.ParseFloat(val, 64); err == nil {
			return json.RawMessage(val)
		}
		return json.RawMessage(`0`)
	case reflect.Struct:
		// For struct types (like basic.Time), check if it's a valid JSON literal
		if isValidJSONLiteral(val) {
			return json.RawMessage(val)
		}
		// Quote as string (e.g. time format strings)
		quoted, _ := json.Marshal(val)
		return quoted
	default:
		// Unknown kind: try as JSON literal, fallback to quoted string
		if isValidJSONLiteral(val) {
			return json.RawMessage(val)
		}
		quoted, _ := json.Marshal(val)
		return quoted
	}
}

// isValidJSONLiteral checks if s is a valid JSON value (number, bool, null, object, array, or quoted string).
func isValidJSONLiteral(s string) bool {
	if len(s) == 0 {
		return false
	}
	switch s[0] {
	case '"', '{', '[', 't', 'f', 'n':
		return json.Valid([]byte(s))
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return json.Valid([]byte(s))
	}
	return false
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
