// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

//go:build !windows

package netpoll

import (
	"context"
	"testing"
	"time"

	"github.com/caiflower/common-tools/web/common/config"
	"github.com/caiflower/common-tools/web/network"
	"github.com/stretchr/testify/assert"
)

func getListenerAddr(trans network.Transporter) string {
	return trans.(*transporter).Listener().Addr().String()
}

func TestDial(t *testing.T) {
	t.Run("NetpollDial", func(t *testing.T) {
		const nw = "tcp"
		var addr = "127.0.0.1:0"
		transporter := NewTransporter(&config.Options{
			Addr:    addr,
			Network: nw,
		})
		go transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
			return nil
		})
		defer transporter.Close()
		time.Sleep(100 * time.Millisecond)

		dial := NewDialer()
		// DialConnection
		_, err := dial.DialConnection("tcp", "localhost:10101", time.Second, nil) // wrong addr
		assert.NotNil(t, err)

		addr = getListenerAddr(transporter)
		nwConn, err := dial.DialConnection(nw, addr, time.Second, nil)
		assert.Nil(t, err)
		defer nwConn.Close()
		_, err = nwConn.Write([]byte("abcdef"))
		assert.Nil(t, err)
		// DialTimeout
		nConn, err := dial.DialTimeout(nw, addr, time.Second, nil)
		assert.Nil(t, err)
		defer nConn.Close()
	})
}
