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

package protocol

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/caiflower/common-tools/pkg/bytebufferpool"
	"github.com/caiflower/common-tools/web/common/nocopy"
	"github.com/caiflower/common-tools/web/network"
)

type Response struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	Headers http.Header

	statusCode int

	conn network.Conn

	// response buffer
	bytebufferpool.ByteBuffer
}

func (resp *Response) Header() http.Header {
	return resp.Headers
}

func (resp *Response) Write(data []byte) (int, error) {
	return resp.ByteBuffer.Write(data)
}

func (resp *Response) Bytes() []byte {
	return append(resp.getHeaderBytes(), resp.ByteBuffer.Bytes()...)
}

func (resp *Response) WriteHeader(statusCode int) {
	resp.statusCode = statusCode
}

func (resp *Response) Reset() {
	resp.ByteBuffer.Reset()
	resp.statusCode = http.StatusOK
	resp.Headers = make(map[string][]string)
}

func (resp *Response) getHeaderBytes() []byte {
	statusText := http.StatusText(resp.statusCode)
	if statusText == "" {
		statusText = "Internal Server Error"
	}

	responseLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", resp.statusCode, statusText)

	var headerBuf bytes.Buffer
	headerBuf.WriteString(responseLine)

	for key, values := range resp.Headers {
		for _, value := range values {
			if key == "Content-Length" {
				continue
			}
			headerBuf.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	if resp.Headers.Get("Date") == "" {
		headerBuf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(http.TimeFormat)))
	}

	if resp.Headers.Get("Server") == "" {
		headerBuf.WriteString("Server: caiflower\r\n")
	}

	headerBuf.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n", resp.ByteBuffer.Len()))
	return headerBuf.Bytes()
}

func (resp *Response) SetConn(conn network.Conn) {
	resp.conn = conn
}
