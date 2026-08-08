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
	"fmt"
	"net/url"
	"strings"
	"time"

	webclient "github.com/caiflower/common-tools/web/app/client"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router"
)

// Client executes routes over HTTP.
type Client struct {
	Server  string
	Token   string
	Headers []string
}

// Execute sends one registered route request and returns the response body.
func (c *Client) Execute(ctx context.Context, route router.RouteInfo, values map[string]string, body []byte) ([]byte, error) {
	if c.Server == "" {
		return nil, fmt.Errorf("server address is empty, run %q or pass --server", "serve")
	}

	httpClient, err := webclient.NewClient(webclient.WithClientReadTimeout(10 * time.Second))
	if err != nil {
		return nil, err
	}

	req := &protocol.Request{}
	req.SetMethod(route.Method)
	req.SetRequestURI(c.requestURL(route, values))
	req.SetHeader("Accept-Encoding", "identity")
	for _, header := range c.Headers {
		key, value, ok := strings.Cut(header, "=")
		if !ok {
			return nil, fmt.Errorf("invalid header %q, expected key=value", header)
		}
		req.SetHeader(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	if c.Token != "" {
		req.SetHeader("Authorization", "Bearer "+c.Token)
	}
	for _, param := range route.Params {
		if param.Source != "header" {
			continue
		}
		if value := values[param.Name]; value != "" {
			req.SetHeader(param.Name, value)
		}
	}
	if len(body) > 0 {
		req.SetBody(body)
		req.Header.SetContentTypeBytes([]byte("application/json"))
	}

	resp := &protocol.Response{}
	if err := httpClient.Do(ctx, req, resp); err != nil {
		return nil, err
	}
	out := append([]byte(nil), resp.Body()...)
	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode(), string(out))
	}
	return out, nil
}

func (c *Client) requestURL(route router.RouteInfo, values map[string]string) string {
	segments := strings.Split(route.Path, "/")
	for i, segment := range segments {
		switch {
		case strings.HasPrefix(segment, ":"):
			if value, ok := values[segment[1:]]; ok {
				segments[i] = url.PathEscape(value)
			}
		case strings.HasPrefix(segment, "*"):
			if value, ok := values[segment[1:]]; ok {
				parts := strings.Split(value, "/")
				for j := range parts {
					parts[j] = url.PathEscape(parts[j])
				}
				segments[i] = strings.Join(parts, "/")
			}
		}
	}
	path := strings.Join(segments, "/")

	query := make(url.Values)
	for _, param := range route.Params {
		if param.Source != "query" {
			continue
		}
		if value := values[param.Name]; value != "" {
			query.Set(param.Name, value)
		}
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return strings.TrimRight(c.Server, "/") + path
}
