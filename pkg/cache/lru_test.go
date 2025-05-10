package cache

import (
	"fmt"
	"testing"
	"time"
)

func TestNewLRUCache(t *testing.T) {
	cache := NewLRUCache[string, string](2)
	go cache.Put("key1", "value1")
	go cache.Put("key2", "value2")
	go cache.Put("key3", "value3")
	go cache.Put("key1", "value1")
	fmt.Println(cache)

	cache1 := NewLRUCache[string, string](3)
	go cache1.Put("key1", "value1")
	go cache1.Put("key2", "value2")
	go cache1.Put("key3", "value3")
	go cache1.Put("key2", "value2")
	go cache1.Put("key1", "value1")
	go cache1.Put("key3", "value3")

	time.Sleep(1 * time.Second)

	for i := 1; i <= 3; i++ {
		go fmt.Println(cache1.Get(fmt.Sprintf("key%d", i)))
		go fmt.Println(cache1.Size())
	}

	time.Sleep(1 * time.Second)
}
