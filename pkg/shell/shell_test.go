package shell

import (
	"fmt"
	"testing"
)

func TestExec(t *testing.T) {
	exec, err := Exec("ls", "-al")
	if err != nil {
		return
	}
	fmt.Println(exec.Stdout.String())
	fmt.Println("---------------------")

	exec, err = ExecInDir("ls", "/Users/lijinlong100", "-la")
	if err != nil {
		return
	}
	fmt.Println(exec.Stdout.String())
}
