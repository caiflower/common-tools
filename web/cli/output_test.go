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
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintOutputJSON(t *testing.T) {
	body := []byte(`{"requestID":"r1","data":{"id":1,"name":"alice"}}`)
	var buf bytes.Buffer
	err := PrintOutput(&buf, "json", body)
	assert.NoError(t, err)
	assert.Equal(t, string(body), buf.String())
}

func TestPrintOutputYAML(t *testing.T) {
	body := []byte(`{"requestID":"r1","data":{"id":1,"name":"alice"}}`)
	var buf bytes.Buffer
	err := PrintOutput(&buf, "yaml", body)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "requestID: r1")
	assert.Contains(t, buf.String(), "name: alice")
}

func TestPrintOutputTableObject(t *testing.T) {
	body := []byte(`{"requestID":"r1","data":{"id":1,"name":"alice"}}`)
	var buf bytes.Buffer
	err := PrintOutput(&buf, "table", body)
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "id")
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "name")
	assert.Contains(t, out, "alice")
}

func TestPrintOutputTableArray(t *testing.T) {
	body := []byte(`{"requestID":"r1","data":[{"id":1,"name":"alice"},{"id":2,"name":"bob"}]}`)
	var buf bytes.Buffer
	err := PrintOutput(&buf, "table", body)
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "id")
	assert.Contains(t, out, "name")
	assert.Contains(t, out, "alice")
	assert.Contains(t, out, "bob")
}

func TestPrintOutputUnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	err := PrintOutput(&buf, "xml", []byte(`{}`))
	assert.Error(t, err)
	assert.Equal(t, "", strings.TrimSpace(buf.String()))
}
