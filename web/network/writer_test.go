/*
 * Copyright 2022 CloudWeGo Authors
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

package network

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

const (
	size1K = 1024
)

func TestConvertNetworkWriter(t *testing.T) {
	iw := &mockIOWriter{}
	w := NewWriter(iw)
	nw, _ := w.(*networkWriter)

	// Test malloc
	buf, _ := w.Malloc(size1K)
	assert.Equal(t, len(buf), size1K)
	assert.Equal(t, len(nw.caches), 1)
	assert.Equal(t, len(nw.caches[0].data), size1K)
	assert.Equal(t, cap(nw.caches[0].data), size1K)
	err := w.Flush()
	assert.Nil(t, err)
	assert.Equal(t, size1K, iw.WriteNum)
	assert.Equal(t, len(nw.caches), 0)
	assert.Equal(t, cap(nw.caches), 1)

	// Test malloc left size
	buf, _ = w.Malloc(size1K + 1)
	assert.Equal(t, len(buf), size1K+1)
	assert.Equal(t, len(nw.caches), 1)
	assert.Equal(t, len(nw.caches[0].data), size1K+1)
	assert.Equal(t, cap(nw.caches[0].data), size1K*2)
	buf, _ = w.Malloc(size1K / 2)
	assert.Equal(t, len(buf), size1K/2)
	assert.Equal(t, len(nw.caches), 1)
	assert.Equal(t, len(nw.caches[0].data), size1K+1+size1K/2)
	assert.Equal(t, cap(nw.caches[0].data), size1K*2)
	buf, _ = w.Malloc(size1K / 2)
	assert.Equal(t, len(buf), size1K/2)
	assert.Equal(t, len(nw.caches), 2)
	assert.Equal(t, len(nw.caches[0].data), size1K+1+size1K/2)
	assert.Equal(t, cap(nw.caches[0].data), size1K*2)
	assert.Equal(t, len(nw.caches[1].data), size1K/2)
	assert.Equal(t, cap(nw.caches[1].data), size1K/2)
	err = w.Flush()
	assert.Nil(t, err)
	assert.Equal(t, size1K*3+1, iw.WriteNum)
	assert.Equal(t, len(nw.caches), 0)
	assert.Equal(t, cap(nw.caches), 2)

	// Test WriteBinary after Malloc
	buf, _ = w.Malloc(size1K * 6)
	assert.Equal(t, len(buf), size1K*6)
	assert.Equal(t, len(nw.caches[0].data), size1K*6)
	b := make([]byte, size1K)
	w.WriteBinary(b)
	assert.Equal(t, size1K*3+1, iw.WriteNum)
	assert.Equal(t, len(nw.caches[0].data), size1K*7)
	assert.Equal(t, cap(nw.caches[0].data), size1K*8)

	b = make([]byte, size1K*4)
	w.WriteBinary(b)
	assert.Equal(t, len(nw.caches[1].data), size1K*4)
	assert.Equal(t, cap(nw.caches[1].data), size1K*4)
	assert.Equal(t, nw.caches[1].readOnly, true)
	w.Flush()
	assert.Equal(t, size1K*14+1, iw.WriteNum)
}

type mockIOWriter struct {
	WriteNum int
}

func (m *mockIOWriter) Write(p []byte) (n int, err error) {
	m.WriteNum += len(p)
	return len(p), nil
}
