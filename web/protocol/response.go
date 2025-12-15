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
)

type Response struct {
	noCopy nocopy.NoCopy //lint:ignore U1000 until noCopy is used

	Headers http.Header

	statusCode int
	headerSent bool

	// response buffer
	bytebufferpool.ByteBuffer
}

func (resp *Response) Header() http.Header {
	return resp.Headers
}

//// flushBuffer flushBuffer
//func (resp *Response) flushBuffer() error {
//	if resp.ByteBuffer.Len() <= 0 {
//		return nil
//	}
//
//	_, err := resp.conn.WriteBinary(resp.ByteBuffer.Bytes())
//	if err != nil {
//		return err
//	}
//
//	return resp.conn.Flush()
//}

func (resp *Response) Write(data []byte) (int, error) {
	if !resp.headerSent {
		resp.WriteHeader(200)
	}
	resp.headerSent = true
	resp.ByteBuffer.Write([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))))
	return resp.ByteBuffer.Write(data)
}

// WriteHeader WriteHeader
func (resp *Response) WriteHeader(statusCode int) {
	if resp.headerSent {
		return
	}

	resp.statusCode = statusCode
	resp.headerSent = true

	// 构建响应行
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "Unknown"
	}

	responseLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText)

	var headerBuf bytes.Buffer
	headerBuf.WriteString(responseLine)

	for key, values := range resp.Headers {
		for _, value := range values {
			if key != "Content-Length" {
				continue
			}
			headerBuf.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	if resp.Headers.Get("Date") == "" {
		headerBuf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(http.TimeFormat)))
	}

	if resp.Headers.Get("Server") == "" {
		headerBuf.WriteString("Server: caiflower/1.0\r\n")
	}

	resp.ByteBuffer.Write(headerBuf.Bytes())
}

func (resp *Response) Reset() {
	resp.ByteBuffer.Reset()
	resp.statusCode = 0
	resp.headerSent = false
	resp.Headers = make(map[string][]string)
}
