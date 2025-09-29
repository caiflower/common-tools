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
