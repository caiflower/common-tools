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
	"io"
	"strings"
	"time"

	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/router"
)

// metadataRoutes picks the route metadata used to generate commands.
// With --server it prefers metadata fetched from that server; without it, or
// when remote discovery fails, local engine metadata is used.
func metadataRoutes(engine *web.Engine, args []string, stderr io.Writer) []router.RouteInfo {
	server := serverFromArgs(args)
	if server == "" || isStaticCommand(commandNameFromArgs(args)) {
		return engine.Handler().Routes()
	}
	metadata, err := Discover(context.Background(), server, cacheOptionsFromArgs(args, stderr))
	if err != nil {
		fmt.Fprintf(stderr, "warning: failed to fetch remote routes from %s, falling back to local route metadata: %v\n", server, err)
		return engine.Handler().Routes()
	}
	return metadata.Routes
}

func serverFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--server":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(arg, "--server="):
			return strings.TrimPrefix(arg, "--server=")
		}
	}
	return ""
}

func commandNameFromArgs(args []string) string {
	for _, arg := range args {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func isStaticCommand(name string) bool {
	switch name {
	case "serve", "routes", "help", "completion":
		return true
	}
	return false
}

func cacheOptionsFromArgs(args []string, stderr io.Writer) CacheOptions {
	opts := CacheOptions{
		TTL:    defaultCacheTTL,
		Stderr: stderr,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--cache-ttl":
			if i+1 < len(args) {
				if ttl, err := time.ParseDuration(args[i+1]); err == nil {
					opts.TTL = ttl
				}
				i++
			}
		case strings.HasPrefix(arg, "--cache-ttl="):
			if ttl, err := time.ParseDuration(strings.TrimPrefix(arg, "--cache-ttl=")); err == nil {
				opts.TTL = ttl
			}
		case arg == "--cache-dir":
			if i+1 < len(args) {
				opts.Dir = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--cache-dir="):
			opts.Dir = strings.TrimPrefix(arg, "--cache-dir=")
		case arg == "--refresh":
			opts.Refresh = true
		}
	}
	return opts
}
