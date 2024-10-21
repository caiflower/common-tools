package nio

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type data struct {
	Age  int
	Name string
}

type dataRes struct {
	ID int64
}

const (
	dataFlag uint8 = iota
	dataResFlag
)

func TestNio(t *testing.T) {
	go func() {

		var id int64 = 0
		server := NewServer(&Config{
			Addr: "127.0.0.1:9090",
		}, &Handler{
			OnSessionConnected: func(session *Session) {
				fmt.Printf("server [%d] OnSessionConnected \n", session.id)
			},
			OnMessageReceived: func(session *Session, msg *Msg) {
				if msg.flag != dataFlag {
					panic("server Unknown msg flag")
				}
				d := &data{}
				if err := msg.Unmarshal(d); err != nil {
					panic("server unmarshal failed, err:" + err.Error())
				}

				fmt.Printf("server [%d] flag = %d data = %+v \n", session.id, msg.flag, d)

				if err := session.WriteMsg(NewMsg(dataResFlag, &dataRes{
					ID: atomic.AddInt64(&id, 1),
				})); err != nil {
					fmt.Printf("server [%d] WriteMsg failed, err: %s\n", session.id, err.Error())
				}
			},
			OnException: func(session *Session) {
				fmt.Println("server OnException")
			},
			OnSessionClosed: func(session *Session) {
				fmt.Printf("server [%d] OnSessionClosed \n", session.id)
			},
		})

		err := server.Open()
		if err != nil {
			panic(err)
		}

		time.Sleep(time.Second * 10)
		server.Close()
	}()

	for i := 0; i < 10; i++ {
		go func() {
			time.Sleep(1 * time.Second)
			client := NewClient(&Config{
				Addr: "127.0.0.1:9090",
			}, &Handler{
				OnSessionConnected: func(session *Session) {
					fmt.Printf("client [%d] OnSessionConnected \n", session.id)
					if err := session.WriteMsg(&Msg{
						flag: dataFlag,
						body: &data{
							Age:  1,
							Name: "Name1",
						},
					}); err != nil {
						fmt.Printf("client [%d] WriteMsg failed, err: %s\n", session.id, err.Error())
					}
				},
				OnMessageReceived: func(session *Session, msg *Msg) {
					if msg.flag != dataResFlag {
						panic("client Unknown msg flag")
					}
					d := &dataRes{}
					if err := msg.Unmarshal(d); err != nil {
						panic("client unmarshal failed, err:" + err.Error())
					}

					fmt.Printf("client [%d] flag = %d data = %+v \n", session.id, msg.flag, d)

					if err := session.WriteMsg(NewMsg(dataFlag, &data{
						Age:  int(d.ID + 1),
						Name: "sessionId:" + strconv.Itoa(int(session.id)) + ":" + strconv.Itoa(int(d.ID+1)),
					})); err != nil {
						fmt.Printf("client [%d] WriteMsg failed, err: %s\n", session.id, err.Error())
					}
				},
				OnException: func(session *Session) {
					fmt.Println("client OnException")
				},
				OnSessionClosed: func(session *Session) {
					fmt.Printf("client [%d] OnSessionClosed \n", session.id)
				},
			})

			err := client.Connect()
			if err != nil {
				panic(err)
			}

			time.Sleep(2 * time.Second)
			client.Close()
		}()
	}

	time.Sleep(3600 * time.Second)
}
