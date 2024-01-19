package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	delayque "github.com/caiflower/common-tools/pkg/delay-queque"
)

type MyData struct {
	Name string
}

func main() {
	wait := sync.WaitGroup{}
	queue := delayque.New()
	total := 10
	consumerCnt := 3

	wait.Add(total)
	for i := 0; i < consumerCnt; i++ {
		go func() {
			for {
				d := queue.Take()
				data := d.GetData().(MyData)
				fmt.Printf("now: %v, dataName: %v\n", time.Now().Format(time.DateTime), data.Name)
				wait.Done()
			}

		}()
	}

	fmt.Printf("offer time: %v\n", time.Now().Format(time.DateTime))
	for i := 0; i < total; i++ {
		queue.Offer(delayque.NewDelayItem(MyData{Name: strconv.Itoa(i)}, time.Now().Add(time.Second*3)))
	}

	wait.Wait()
}
