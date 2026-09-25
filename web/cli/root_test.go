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
	"testing"

	"github.com/caiflower/common-tools/global"
	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/app/server/config"
	"github.com/stretchr/testify/assert"
)

type fakeResourceManager struct {
	added    bool
	signaled bool
}

func (f *fakeResourceManager) AddDaemonWithOrder(global.DaemonResource, int) {
	f.added = true
}

func (f *fakeResourceManager) Signal() {
	f.signaled = true
}

func newCLIEngine() *web.Engine {
	return web.Default(
		config.WithName("cli-test"),
		config.WithAddr(":0"),
		config.WithMode(config.ServerModeStandard),
	)
}

func TestNewRootCommandName(t *testing.T) {
	root := New(newCLIEngine(), WithName("myapp"))
	assert.Equal(t, "myapp", root.Use)
}

func TestServeUsesResourceManager(t *testing.T) {
	manager := &fakeResourceManager{}
	root := New(newCLIEngine(), WithResourceManager(manager))
	root.SetArgs([]string{"serve"})
	err := root.Execute()
	assert.NoError(t, err)
	assert.True(t, manager.added)
	assert.True(t, manager.signaled)
}

func TestRunReturnsRootExecution(t *testing.T) {
	root := New(newCLIEngine(), WithName("myapp"))
	root.SetArgs([]string{})
	err := Run(newCLIEngine(), WithName("myapp"))
	assert.NoError(t, err)
	assert.Equal(t, "myapp", root.Use)
}
