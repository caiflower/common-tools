package cache

import (
	"fmt"
	"testing"
)

func TestLFUCache(t *testing.T) {
	cache := NewLFUCache(2)

	cache.Put("key1", "value1")
	cache.Put("key2", "value2")
	cache.Put("key1", "value1")
	cache.Put("key3", "value3")

	fmt.Println(cache)

	cache.Put("key3", "value3")
	fmt.Println(cache)

	cache.Put("key2", "key2")

	fmt.Println(cache)
}
