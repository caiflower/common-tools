/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
