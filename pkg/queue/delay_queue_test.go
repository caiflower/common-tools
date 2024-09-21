package queue

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type MyData struct {
	Name     string
	Time     time.Time
	TakeTime time.Time
}

var count int64

func TestNewConcurrent(t *testing.T) {
	queue := NewDelayQueue()
	consumerCnt := 10
	producerCnt := 10
	preCnt := 10
	total := producerCnt * preCnt

	ch := make(chan int)
	for i := 0; i <= consumerCnt; i++ {
		go func() {
			for {
				di := queue.Take()
				data := di.GetData()
				myData := data.(MyData)
				myData.TakeTime = time.Now()
				atomic.AddInt64(&count, 1)
				fmt.Printf("takeNow=%v, time=%v, name=%v \n",
					myData.TakeTime.Format(time.DateTime),
					myData.Time.Format(time.DateTime),
					myData.Name)
				if count == int64(total) {
					close(ch)
				}
			}
		}()
	}

	for i := 0; i < producerCnt; i++ {
		go func(producerNum int) {
			fmt.Printf("producer:%v ,begin offer time=%v\n", producerNum, time.Now().Format(time.DateTime))
			for j := 1; j <= preCnt; j++ {
				duration := time.Second * 3
				t := time.Now().Add(duration)
				queue.Offer(NewDelayItem(MyData{Name: "producer" + strconv.Itoa(producerNum) + ",num" + strconv.Itoa(j), Time: t}, t))
				time.Sleep(time.Second)
			}
		}(i)
	}

	<-ch
}

func BenchmarkNew(t *testing.B) {
	for i := 0; i < t.N; i++ {
		queue := NewDelayQueue()
		total := 10 * i

		fmt.Printf("offer time=%v, total: %v\n", time.Now().Format(time.DateTime), total)
		for i := 1; i <= total; i++ {
			duration := time.Second * 3
			t := time.Now().Add(duration)
			queue.Offer(NewDelayItem(MyData{Name: strconv.Itoa(i), Time: t}, t))
			time.Sleep(time.Second)
		}

		for i := 1; i <= total; i++ {
			di := queue.Take()
			data := di.GetData()
			myData := data.(MyData)
			myData.TakeTime = time.Now()
			atomic.AddInt64(&count, 1)
			fmt.Printf("takeNow=%v, time=%v, name=%v \n",
				myData.TakeTime.Format(time.DateTime),
				myData.Time.Format(time.DateTime),
				myData.Name)
		}
	}
}
