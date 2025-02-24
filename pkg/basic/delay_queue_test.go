package basic

import (
	"fmt"
	"testing"
	"time"
)

type testDelayQueue struct {
	Name string
}

func TestDelayQueue(t *testing.T) {
	queue := NewDelayQueue()
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
	queue.Add(&testDelayQueue{Name: "10"}, time.Now().Add(10*time.Second))
	queue.Add(&testDelayQueue{Name: "8"}, time.Now().Add(8*time.Second))
	go func() {
		for {
			value := queue.Take()
			fmt.Printf("time = %s, value = %s\n", time.Now().Format("2006-01-02 15:04:05"), value.(*testDelayQueue).Name)
		}
	}()
	queue.Add(&testDelayQueue{Name: "5"}, time.Now().Add(5*time.Second))
	queue.Add(&testDelayQueue{Name: "4"}, time.Now().Add(4*time.Second))
	queue.Add(&testDelayQueue{Name: "3"}, time.Now().Add(3*time.Second))

	time.Sleep(20 * time.Second)

}
