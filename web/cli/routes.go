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
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/caiflower/common-tools/web"
	"github.com/caiflower/common-tools/web/router"
	"github.com/spf13/cobra"
)

func routesCommand(engine *web.Engine, cfg *options) *cobra.Command {
	return &cobra.Command{
		Use:   "routes",
		Short: "List available routes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, _ := cmd.Flags().GetString("server")
			format, _ := cmd.Flags().GetString("output")
			if server == "" {
				routes := engine.Handler().Routes()
				return printRoutes(cmd.OutOrStdout(), format, Metadata{
					Name:      cfg.name,
					Resources: resourceNames(routes),
					Routes:    routes,
				})
			}
			metadata, err := Discover(cmd.Context(), server, cacheOptionsFromFlags(cmd))
			if err != nil {
				return err
			}
			return printRoutes(cmd.OutOrStdout(), format, metadata)
		},
	}
}

func cacheOptionsFromFlags(cmd *cobra.Command) CacheOptions {
	ttl, _ := cmd.Flags().GetDuration("cache-ttl")
	dir, _ := cmd.Flags().GetString("cache-dir")
	refresh, _ := cmd.Flags().GetBool("refresh")
	return CacheOptions{TTL: ttl, Dir: dir, Refresh: refresh}
}

func printRoutes(w io.Writer, format string, metadata Metadata) error {
	if format == "table" {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "METHOD\tPATH\tOPERATION\tRESOURCE\tVERB"); err != nil {
			return err
		}
		for _, route := range metadata.Routes {
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				route.Method, route.Path, route.OperationID, route.Resource, route.Verb); err != nil {
				return err
			}
		}
		return tw.Flush()
	}

	body, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return PrintOutput(w, format, body)
}

func resourceNames(routes []router.RouteInfo) []string {
	seen := make(map[string]struct{})
	var names []string
	for _, route := range routes {
		if route.Resource == "" {
			continue
		}
		if _, ok := seen[route.Resource]; ok {
			continue
		}
		seen[route.Resource] = struct{}{}
		names = append(names, route.Resource)
	}
	return names
}
