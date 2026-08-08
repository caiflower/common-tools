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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	webclient "github.com/caiflower/common-tools/web/app/client"
	"github.com/caiflower/common-tools/web/protocol"
	"github.com/caiflower/common-tools/web/router"
)

const cliRoutesPath = "/cli/routes"

// Metadata is the payload returned by GET /cli/routes.
type Metadata struct {
	Name      string             `json:"name"`
	Version   string             `json:"version"`
	Resources []string           `json:"resources"`
	Routes    []router.RouteInfo `json:"routes"`
}

// CacheOptions controls remote metadata discovery and local caching.
type CacheOptions struct {
	TTL     time.Duration
	Dir     string
	Refresh bool
	Stderr  io.Writer
}

// Discover fetches route metadata from a running server and caches it locally.
func Discover(ctx context.Context, server string, opts CacheOptions) (Metadata, error) {
	if server == "" {
		return Metadata{}, fmt.Errorf("server address is empty")
	}
	if opts.TTL <= 0 {
		opts.TTL = defaultCacheTTL
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	server = normalizeServer(server)

	path, err := cacheFilePath(server, opts.Dir)
	if err != nil {
		return Metadata{}, err
	}
	cached, err := readCacheFile(path)
	if err != nil {
		return Metadata{}, err
	}
	if cached != nil && !opts.Refresh && time.Since(cached.FetchedAt) < opts.TTL {
		return cached.Metadata, nil
	}

	if cached != nil {
		metadata, status, fetchErr := fetchMetadata(ctx, server, cached.Version)
		if fetchErr != nil {
			fmt.Fprintf(opts.Stderr, "warning: refresh failed, using cached metadata: %v\n", fetchErr)
			return cached.Metadata, nil
		}
		if status == 304 {
			cached.FetchedAt = time.Now()
			if err := writeCacheFile(path, cached.Metadata, cached.FetchedAt); err != nil {
				return Metadata{}, err
			}
			return cached.Metadata, nil
		}
		if err := writeCacheFile(path, metadata, time.Now()); err != nil {
			return Metadata{}, err
		}
		return metadata, nil
	}

	metadata, status, err := fetchMetadata(ctx, server, "")
	if err != nil {
		return Metadata{}, err
	}
	if status == 304 {
		return Metadata{}, fmt.Errorf("server returned not modified but no local cache exists")
	}
	if err := writeCacheFile(path, metadata, time.Now()); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func fetchMetadata(ctx context.Context, server, version string) (Metadata, int, error) {
	httpClient, err := webclient.NewClient(webclient.WithClientReadTimeout(10 * time.Second))
	if err != nil {
		return Metadata{}, 0, err
	}
	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI(server + cliRoutesPath)
	req.SetHeader("Accept-Encoding", "identity")
	if version != "" {
		req.SetHeader("If-None-Match", `"v1-`+version+`"`)
	}
	resp := &protocol.Response{}
	if err := httpClient.Do(ctx, req, resp); err != nil {
		return Metadata{}, 0, err
	}
	if resp.StatusCode() == 304 {
		return Metadata{}, 304, nil
	}
	if resp.StatusCode() >= 400 {
		return Metadata{}, resp.StatusCode(), fmt.Errorf("fetch /cli/routes failed with status %d: %s", resp.StatusCode(), string(resp.Body()))
	}
	var metadata Metadata
	if err := json.Unmarshal(resp.Body(), &metadata); err != nil {
		return Metadata{}, resp.StatusCode(), err
	}
	return metadata, resp.StatusCode(), nil
}

func normalizeServer(server string) string {
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	return strings.TrimRight(server, "/")
}
