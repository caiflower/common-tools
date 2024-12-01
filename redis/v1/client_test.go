package v1

import (
	"fmt"
	"testing"
	"time"
)

type TestObject struct {
	Name string
	Age  int
}

func TestNewRedisClient(t *testing.T) {
	client := NewRedisClient(Config{
		Addrs:    []string{"10.226.138.162:6379"},
		Password: "CloudIaas@123",
		DB:       8,
	})
	if client == nil {
		panic("client init failed")
	}

	if err := client.Set("test", "test"); err != nil {
		panic(err)
	}

	if getString, err := client.GetString("test"); err != nil {
		panic(err)
	} else {
		fmt.Println(getString)
	}

	if err := client.Del("test"); err != nil {
		panic(err)
	}

	if err := client.Set("object", &TestObject{Age: 1, Name: "testObject"}); err != nil {
		panic(err)
	}

	object := &TestObject{}
	if err := client.Get("object", object); err != nil {
		panic(err)
	} else {
		fmt.Printf("object = %v\n", object)
	}

	if err := client.Del("object"); err != nil {
		panic(err)
	}

	if err := client.SetPeriod("objectTTL", &TestObject{Age: 1, Name: "testObject"}, time.Second*60); err != nil {
		panic(err)
	}

	if err := client.HSet("ObjectH", "key", &TestObject{Age: 1, Name: "testObject"}); err != nil {
		panic(err)
	}

	if err := client.HSet("ObjectH", map[string]interface{}{"key1": &TestObject{Age: 1, Name: "testObject"}}); err != nil {
		panic(err)
	}

	if err := client.HSet("ObjectH", map[string]string{"key2": "string"}); err != nil {
		panic(err)
	}

	if err := client.HGet("ObjectH", "key", object); err != nil {
		panic(err)
	} else {
		fmt.Printf("object = %v\n", object)
	}

	if err := client.HGet("ObjectH", "key1", object); err != nil {
		panic(err)
	} else {
		fmt.Printf("object = %v\n", object)
	}

	if v, err := client.HGetString("ObjectH", "key2"); err != nil {
		panic(err)
	} else {
		fmt.Printf("string = %v\n", v)
	}

	if err := client.Del("ObjectH"); err != nil {
		panic(err)
	}
}
