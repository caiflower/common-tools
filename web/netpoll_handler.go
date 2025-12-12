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

package web

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"time"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/caiflower/common-tools/pkg/tools"
	"github.com/caiflower/common-tools/web/common/e"
	"github.com/cloudwego/netpoll"
)

// NetpollHttpHandler netpoll HTTP 处理器
type NetpollHttpHandler struct {
	server *NetpollHttpServer
	conn   netpoll.Connection
}

// ServeHTTP 处理 HTTP 请求
func (h *NetpollHttpHandler) ServeHTTP() {
	// 从响应池获取响应写入器
	w := h.server.responsePool.Get().(*NetpollResponseWriter)
	defer h.server.responsePool.Put(w)

	// 重置响应写入器
	w.conn = h.conn
	w.server = h.server
	w.statusCode = 0
	w.headerSent = false
	w.bytesWritten = 0
	w.bufferPos = 0
	if w.header == nil {
		w.header = make(http.Header)
	} else {
		// 清空 header
		for k := range w.header {
			delete(w.header, k)
		}
	}

	// 读取请求
	req, err := h.readRequestOptimized()
	if err != nil {
		h.server.logger.Error("Failed to read request: %v", err)
		return
	}

	// 设置 trace ID
	var traceID string
	if h.server.config.HeaderTraceID != "" {
		traceID = req.Header.Get(h.server.config.HeaderTraceID)
	}
	if traceID == "" {
		traceID = tools.UUID()
		if h.server.config.HeaderTraceID != "" {
			req.Header.Set(h.server.config.HeaderTraceID, traceID)
		}
	}
	golocalv1.PutTraceID(traceID)
	golocalv1.Put(beginTime, time.Now())
	golocalv1.PutContext(req.Context())
	defer golocalv1.Clean()

	// 处理特殊请求
	if h.server.handler.specialRequest(w, req) {
		golocalv1.Clean()
	}

	// 限流检查
	if h.server.config.WebLimiter.Enable && h.server.limiterBucket != nil {
		if !h.server.limiterBucket.TakeTokenNonBlocking() {
			res := commonResponse{
				RequestId: traceID,
				Error:     e.NewApiError(e.TooManyRequests, "TooManyRequests", nil),
			}
			h.writeErrorResponse(w, res)
		}
	}

	// 处理业务请求
	h.server.handler.dispatch(w, req)

	// 检查是否需要保持连接
	if !h.shouldKeepAlive(req) {
		return
	}
}

// readRequestOptimized 优化的 HTTP 请求读取
func (h *NetpollHttpHandler) readRequestOptimized() (req *http.Request, err error) {
	buffer := h.server.bufferPool.Get().([]byte)
	defer h.server.bufferPool.Put(buffer)

	// 重用 bufio.Reader，避免频繁分配
	reader := netpoll.NewIOReader(h.conn.Reader())
	bufReader := bufio.NewReaderSize(reader, len(buffer))

	// 解析请求，增加重试机制
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		req, err = http.ReadRequest(bufReader)
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			return nil, err
		}
		// 短暂等待后重试
		time.Sleep(time.Microsecond * 100)
	}

	// 设置远程地址
	if req.RemoteAddr == "" {
		if addr := h.conn.RemoteAddr(); addr != nil {
			req.RemoteAddr = addr.String()
		}
	}

	return req, nil
}

// shouldKeepAlive 检查是否应该保持连接
func (h *NetpollHttpHandler) shouldKeepAlive(req *http.Request) bool {
	if req.ProtoMajor < 1 || (req.ProtoMajor == 1 && req.ProtoMinor < 1) {
		return false
	}

	if req.Header.Get("Connection") == "close" {
		return false
	}

	if req.ProtoMajor == 1 && req.ProtoMinor == 0 {
		return req.Header.Get("Connection") == "keep-alive"
	}

	return true
}

func (h *NetpollHttpHandler) writeErrorResponse(w *NetpollResponseWriter, res commonResponse) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(res.Error.GetCode())

	_bytes, _ := tools.Marshal(res)
	_, _ = w.Write(_bytes)
}

// NetpollResponseWriter netpoll 响应写入器
type NetpollResponseWriter struct {
	conn         netpoll.Connection
	header       http.Header
	statusCode   int
	headerSent   bool
	server       *NetpollHttpServer
	bytesWritten int64
	buffer       []byte // 添加缓冲区
	bufferPos    int    // 缓冲区位置
}

// Header 返回响应头
func (w *NetpollResponseWriter) Header() http.Header {
	return w.header
}

// Write 写入响应体
func (w *NetpollResponseWriter) Write(data []byte) (int, error) {
	if !w.headerSent {
		w.WriteHeader(http.StatusOK)
	}

	totalWritten := 0
	remaining := data

	for len(remaining) > 0 {
		// 计算可写入缓冲区的数据量
		available := cap(w.buffer) - w.bufferPos
		if available == 0 {
			// 缓冲区满，刷新
			if err := w.flushBuffer(); err != nil {
				return totalWritten, err
			}
			// 刷新后重新计算可用空间
			available = cap(w.buffer) - w.bufferPos
		}

		// 写入数据到缓冲区
		writeSize := len(remaining)
		if writeSize > available {
			writeSize = available
		}

		// 防止死循环：如果 writeSize 为 0，直接写入网络
		if writeSize == 0 {
			break
		}

		copy(w.buffer[w.bufferPos:], remaining[:writeSize])
		w.bufferPos += writeSize
		totalWritten += writeSize
		remaining = remaining[writeSize:]
	}

	w.bytesWritten += int64(totalWritten)

	// 如果数据较大或缓冲区接近满，立即刷新
	if err := w.flushBuffer(); err != nil {
		return totalWritten, err
	}

	return totalWritten, nil
}

// flushBuffer 刷新缓冲区
func (w *NetpollResponseWriter) flushBuffer() error {
	if w.bufferPos == 0 {
		return nil
	}

	_, err := w.conn.Writer().WriteBinary(w.buffer[:w.bufferPos])
	if err != nil {
		return err
	}

	// 重置缓冲区位置
	w.bufferPos = 0

	// 刷新网络缓冲区
	return w.conn.Writer().Flush()
}

// WriteHeader 写入状态码和响应头
func (w *NetpollResponseWriter) WriteHeader(statusCode int) {
	if w.headerSent {
		return
	}

	w.statusCode = statusCode
	w.headerSent = true

	// 构建响应行
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "Unknown"
	}

	responseLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText)

	// 构建响应头
	var headerBuf bytes.Buffer
	headerBuf.WriteString(responseLine)

	// 写入自定义头部
	for key, values := range w.header {
		for _, value := range values {
			headerBuf.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// 添加默认头部
	if w.header.Get("Date") == "" {
		headerBuf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(http.TimeFormat)))
	}

	if w.header.Get("Server") == "" {
		headerBuf.WriteString("Server: netpoll-http/1.0\r\n")
	}

	// 结束头部
	headerBuf.WriteString("\r\n")

	// 写入响应头
	writer := w.conn.Writer()
	_, _ = writer.WriteBinary(headerBuf.Bytes())
}

// Flush 刷新缓冲区
func (w *NetpollResponseWriter) Flush() {
	// 先刷新内部缓冲区
	if err := w.flushBuffer(); err != nil {
		// 记录错误但不返回，保持接口兼容性
		if w.server != nil && w.server.logger != nil {
			w.server.logger.Error("Failed to flush buffer: %v", err)
		}
	}

	// 然后刷新连接缓冲区
	if flusher := w.conn.Writer(); flusher != nil {
		flusher.Flush()
	}
}

// CloseNotify 连接关闭通知（兼容性方法）
func (w *NetpollResponseWriter) CloseNotify() <-chan bool {
	ch := make(chan bool, 1)
	go func() {
		// 简单实现，实际应该监听连接状态
		<-time.After(time.Hour) // 默认1小时超时
		ch <- true
	}()
	return ch
}
