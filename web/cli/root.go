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
	"github.com/caiflower/common-tools/web"
	"github.com/spf13/cobra"
)

// New builds the dual-mode cobra root command for the engine.
func New(engine *web.Engine, opts ...Option) *cobra.Command {
	cfg := defaultOptions(engine, opts...)
	root := &cobra.Command{
		Use: cfg.name,
	}
	root.PersistentFlags().String("server", defaultServer(engine), "server address")
	root.PersistentFlags().String("token", "", "bearer token")
	root.PersistentFlags().StringArray("header", nil, "request header, repeatable key=value")
	root.PersistentFlags().String("output", "table", "output format: table, json or yaml")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if client, ok := cfg.runner.(*Client); ok {
			server, _ := cmd.Flags().GetString("server")
			if server != "" {
				client.Server = server
			}
			client.Token, _ = cmd.Flags().GetString("token")
			client.Headers, _ = cmd.Flags().GetStringArray("header")
		}
		return nil
	}
	root.AddCommand(serveCommand(engine, cfg))
	addDynamicCommands(root, engine.Handler().Routes(), cfg.runner)
	return root
}

// Run builds and executes the dual-mode root command.
func Run(engine *web.Engine, opts ...Option) error {
	return New(engine, opts...).Execute()
}
