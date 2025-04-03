package basic

import (
	"fmt"
	"testing"
)

func TestLinkedHashMap(t *testing.T) {
	linkedHashMap := NewLinkHashMap()
	linkedHashMap.Put("key1", "value1")
	linkedHashMap.Put("key2", "value2")
	linkedHashMap.Put("key3", "value3")

	fmt.Println(linkedHashMap)

	get, b := linkedHashMap.Get("key2")
	fmt.Println(get == "value2")
	fmt.Println(b)

	fmt.Println(linkedHashMap)

	linkedHashMap.Remove("key3")
	fmt.Println(linkedHashMap)
	linkedHashMap.Remove("key2")
	fmt.Println(linkedHashMap)
	linkedHashMap.Remove("key1")
	fmt.Println(linkedHashMap)
}
