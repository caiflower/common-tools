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

package cli

import (
	"context"
	"testing"

	"github.com/caiflower/common-tools/web/router"
	"github.com/stretchr/testify/assert"
)

type cliCmdReq struct {
	ID   int    `path:"id" verf:"required"`
	Name string `query:"name"`
	Body string `json:"body"`
}

type cliCmdResp struct{}

func cliCmdGet(req *cliCmdReq) (*cliCmdResp, error) {
	return &cliCmdResp{}, nil
}

type fakeRunner struct {
	route  router.RouteInfo
	values map[string]string
	body   []byte
}

func (f *fakeRunner) Execute(_ context.Context, route router.RouteInfo, values map[string]string, body []byte) ([]byte, error) {
	f.route = route
	f.values = values
	f.body = body
	return []byte(`{"data":"ok"}`), nil
}

func TestDynamicCommandsGenerateResourceFlag(t *testing.T) {
	engine := newCLIEngine()
	engine.GET("/users/:id", cliCmdGet)
	runner := &fakeRunner{}
	root := New(engine, WithName("myapp"), WithRunner(runner))

	cmd, _, err := root.Find([]string{"get", "users"})
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.Equal(t, "users", cmd.Name())

	idFlag := cmd.Flags().Lookup("id")
	assert.NotNil(t, idFlag)
	nameFlag := cmd.Flags().Lookup("name")
	assert.NotNil(t, nameFlag)
}

func TestCallCommandExists(t *testing.T) {
	engine := newCLIEngine()
	engine.GET("/users/:id", cliCmdGet)
	root := New(engine, WithName("myapp"), WithRunner(&fakeRunner{}))

	cmd, _, err := root.Find([]string{"call", "cliCmdGet"})
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
}

func TestResourceCommandExecutesRunner(t *testing.T) {
	engine := newCLIEngine()
	engine.GET("/users/:id", cliCmdGet)
	runner := &fakeRunner{}
	root := New(engine, WithName("myapp"), WithRunner(runner))
	root.SetArgs([]string{"get", "users", "--id=1", "--name=alice"})

	err := root.Execute()
	assert.NoError(t, err)
	assert.Equal(t, "1", runner.values["id"])
	assert.Equal(t, "alice", runner.values["name"])
}

func TestBodyCommandAddsDataFlags(t *testing.T) {
	engine := newCLIEngine()
	engine.POST("/users", func(req *cliCmdReq) (*cliCmdResp, error) {
		return &cliCmdResp{}, nil
	})
	root := New(engine, WithName("myapp"), WithRunner(&fakeRunner{}))

	cmd, _, err := root.Find([]string{"create", "users"})
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("data"))
	assert.NotNil(t, cmd.Flags().Lookup("file"))
}
