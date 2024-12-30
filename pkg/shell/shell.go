package shell

import (
	"bytes"
	"errors"
	"os/exec"
)

type Result struct {
	Stdout bytes.Buffer
	Errout bytes.Buffer
}

// Exec exec os command
func Exec(name string, args ...string) (result *Result, err error) {
	return ExecInDir(name, "", args...)
}

func ExecInDir(name string, dir string, args ...string) (result *Result, err error) {

	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	result = &Result{}
	cmd.Stdout = &result.Stdout
	cmd.Stderr = &result.Errout

	if err = cmd.Start(); err != nil {
		return
	}

	if err = cmd.Wait(); err != nil {
		return
	}

	if errStr := result.Errout.String(); errStr != "" {
		err = errors.New(errStr)
	}

	return
}
