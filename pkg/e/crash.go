package e

import (
	"fmt"
	"runtime/debug"
	"time"
)

func OnError(txt string) {
	if r := recover(); r != nil {
		fmt.Println(time.Now().Format("2006-01-02 15:04:05"), "[ERROR] -", "Got a runtime error %s. %s\n%s", txt, r, string(debug.Stack()))
	}
}
