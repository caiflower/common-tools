package nio

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestNio(t *testing.T) {
	go func() {
		server := NewServer(&Config{
			Addr: "127.0.0.1:9090",
		}, &Handler{
			OnSessionConnected: func(session *Session) {
				fmt.Printf("server %d OnSessionConnected \n", session.id)
			},
			OnMessageReceived: func(session *Session, message *Msg) {
				for i := 0; i < 100; i++ {
					if int(message.flag) == i {
						fmt.Printf("server msg flag = %d, body = %s \n", message.flag, string(message.bytes))
						session.WriteMsg(&Msg{
							flag: uint8(i + 1),
							body: strconv.Itoa(i + 1),
						})
						break
					}
				}
			},
			OnException: func(session *Session) {
				fmt.Println("server OnException")
			},
			OnSessionClosed: func(session *Session) {
				fmt.Printf("server %d OnSessionClosed \n", session.id)
			},
		})

		err := server.Open()
		if err != nil {
			panic(err)
		}

	}()

	go func() {

		time.Sleep(1 * time.Second)
		client := NewClient(&Config{
			Addr: "127.0.0.1:9090",
		}, &Handler{
			OnSessionConnected: func(session *Session) {
				session.WriteMsg(&Msg{
					flag: 1,
					body: "1",
				})
			},
			OnMessageReceived: func(session *Session, message *Msg) {
				for i := 0; i < 100; i++ {
					if int(message.flag) == i {
						fmt.Printf("client msg flag = %d, body = %s \n", message.flag, string(message.bytes))
						session.WriteMsg(&Msg{
							flag: uint8(i + 1),
							body: strconv.Itoa(i + 1),
						})
						break
					}
				}
			},
			OnException: func(session *Session) {
				fmt.Println("server OnException")
			},
		})

		err := client.Connect()
		if err != nil {
			panic(err)
		}

		time.Sleep(2 * time.Second)
		client.Close()
	}()

	time.Sleep(3600 * time.Second)
}
