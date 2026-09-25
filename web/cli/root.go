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
	"os"

	"github.com/caiflower/common-tools/web"
	"github.com/spf13/cobra"
)

// New builds the dual-mode cobra root command for the engine using the
// current process arguments. It prefers remote route metadata when --server
// is present and falls back to local metadata when discovery fails.
func New(engine *web.Engine, opts ...Option) *cobra.Command {
	return NewWithArgs(engine, os.Args[1:], opts...)
}

// NewWithArgs builds the dual-mode cobra root command from explicit arguments.
// When args contain --server, route metadata is fetched from that server first;
// if fetching fails, local metadata is used with a warning.
func NewWithArgs(engine *web.Engine, args []string, opts ...Option) *cobra.Command {
	cfg := defaultOptions(engine, opts...)
	routes := metadataRoutes(engine, args, cfg.stderr)
	root := &cobra.Command{
		Use: cfg.name,
	}
	root.PersistentFlags().String("server", "", "server address")
	root.PersistentFlags().String("token", "", "bearer token")
	root.PersistentFlags().StringArray("header", nil, "request header, repeatable key=value")
	root.PersistentFlags().String("output", "table", "output format: table, json or yaml")
	root.PersistentFlags().Bool("refresh", false, "force refresh remote route metadata")
	root.PersistentFlags().Duration("cache-ttl", defaultCacheTTL, "remote route metadata cache ttl")
	root.PersistentFlags().String("cache-dir", "", "remote route metadata cache directory")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if client, ok := cfg.runner.(*Client); ok {
			server, _ := cmd.Flags().GetString("server")
			if server != "" {
				client.Server = normalizeServer(server)
			}
			client.Token, _ = cmd.Flags().GetString("token")
			client.Headers, _ = cmd.Flags().GetStringArray("header")
		}
		return nil
	}
	root.AddCommand(serveCommand(engine, cfg))
	root.AddCommand(routesCommand(engine, cfg))
	addDynamicCommands(root, routes, cfg.runner)
	return root
}

// Run builds and executes the dual-mode root command.
func Run(engine *web.Engine, opts ...Option) error {
	return New(engine, opts...).Execute()
}
