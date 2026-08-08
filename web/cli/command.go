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
	"os"
	"strings"
	"unicode"

	"github.com/caiflower/common-tools/web/router"
	"github.com/spf13/cobra"
)

type commandRunner interface {
	Execute(ctx context.Context, route router.RouteInfo, values map[string]string, body []byte) ([]byte, error)
}

type routeGroup struct {
	verb     string
	resource string
	routes   []router.RouteInfo
}

type paramFlag struct {
	flag  string
	param string
}

func addDynamicCommands(root *cobra.Command, routes []router.RouteInfo, runner commandRunner) {
	groups := make(map[string]*routeGroup)
	call := &cobra.Command{
		Use:   "call",
		Short: "Call a route by operationID",
	}

	for _, route := range routes {
		if route.Resource != "" && route.Verb != "" {
			key := route.Verb + "\x00" + route.Resource
			group := groups[key]
			if group == nil {
				group = &routeGroup{verb: route.Verb, resource: route.Resource}
				groups[key] = group
			}
			group.routes = append(group.routes, route)
		}
		if runner != nil {
			call.AddCommand(buildCallCommand(route, runner))
		}
	}

	for _, group := range groups {
		if runner != nil {
			root.AddCommand(buildVerbCommand(group, runner))
		}
	}
	if len(call.Commands()) > 0 {
		root.AddCommand(call)
	}
}

func buildVerbCommand(group *routeGroup, runner commandRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   group.verb,
		Short: "Run " + group.verb + " commands",
	}
	for range group.routes {
		cmd.AddCommand(buildResourceCommand(group.resource, group.routes, runner))
		break
	}
	return cmd
}

func buildResourceCommand(resource string, routes []router.RouteInfo, runner commandRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   resource,
		Short: "Call " + resource + " routes",
	}
	paramFlags := addParamFlags(cmd, routes)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRoute(cmd, routes, paramFlags, runner)
	}
	return cmd
}

func buildCallCommand(route router.RouteInfo, runner commandRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   route.OperationID,
		Short: "Call " + route.OperationID,
	}
	paramFlags := addParamFlags(cmd, []router.RouteInfo{route})
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRoute(cmd, []router.RouteInfo{route}, paramFlags, runner)
	}
	return cmd
}

func addParamFlags(cmd *cobra.Command, routes []router.RouteInfo) []paramFlag {
	var paramFlags []paramFlag
	hasBody := false
	for _, route := range routes {
		for _, param := range route.Params {
			if param.Source == "body" {
				hasBody = true
				continue
			}
			name := flagName(param.Name)
			paramFlags = append(paramFlags, paramFlag{flag: name, param: param.Name})
			cmd.Flags().String(name, "", "Request "+param.Source+" parameter "+param.Name)
			if param.Required {
				_ = cmd.MarkFlagRequired(name)
			}
		}
	}
	if hasBody {
		cmd.Flags().String("data", "", "JSON request body")
		cmd.Flags().StringP("file", "f", "", "JSON request body file")
	}
	return paramFlags
}

func runRoute(cmd *cobra.Command, routes []router.RouteInfo, paramFlags []paramFlag, runner commandRunner) error {
	values := make(map[string]string)
	for _, pf := range paramFlags {
		value, _ := cmd.Flags().GetString(pf.flag)
		values[pf.param] = value
	}

	var body []byte
	if cmd.Flags().Changed("data") {
		data, _ := cmd.Flags().GetString("data")
		body = []byte(data)
	} else if cmd.Flags().Changed("file") {
		file, _ := cmd.Flags().GetString("file")
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		body = data
	}

	route := selectRoute(routes, values)
	out, err := runner.Execute(context.Background(), route, values, body)
	if err != nil {
		return err
	}
	format, _ := cmd.Flags().GetString("output")
	return PrintOutput(cmd.OutOrStdout(), format, out)
}

func selectRoute(routes []router.RouteInfo, values map[string]string) router.RouteInfo {
	if len(routes) == 1 {
		return routes[0]
	}
	for _, route := range routes {
		allPresent := true
		for _, param := range route.Params {
			if param.Source == "path" && param.Required && values[param.Name] == "" {
				allPresent = false
				break
			}
		}
		if allPresent {
			return route
		}
	}
	return routes[0]
}

func flagName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}
