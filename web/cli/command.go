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

type verbGroup struct {
	verb      string
	resources map[string][]router.RouteInfo
}

type paramFlag struct {
	flag  string
	param string
}

func addDynamicCommands(root *cobra.Command, routes []router.RouteInfo, runner commandRunner) {
	groups := make(map[string]*verbGroup)
	call := &cobra.Command{
		Use:   "call",
		Short: "Call a route by operationID",
	}

	for _, route := range routes {
		if route.Resource != "" && route.Verb != "" && route.Verb != "call" {
			group := groups[route.Verb]
			if group == nil {
				group = &verbGroup{verb: route.Verb, resources: make(map[string][]router.RouteInfo)}
				groups[route.Verb] = group
			}
			group.resources[route.Resource] = append(group.resources[route.Resource], route)
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

func buildVerbCommand(group *verbGroup, runner commandRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   group.verb,
		Short: "Run " + group.verb + " commands",
	}
	for resource, routes := range group.resources {
		cmd.AddCommand(buildResourceCommand(resource, routes, runner))
	}
	return cmd
}

func buildResourceCommand(resource string, routes []router.RouteInfo, runner commandRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   resource,
		Short: "Call " + resource + " routes",
	}
	paramFlags := addParamFlags(cmd, routes, len(routes) == 1)
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
	paramFlags := addParamFlags(cmd, []router.RouteInfo{route}, true)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRoute(cmd, []router.RouteInfo{route}, paramFlags, runner)
	}
	return cmd
}

func addParamFlags(cmd *cobra.Command, routes []router.RouteInfo, enforceRequired bool) []paramFlag {
	var paramFlags []paramFlag
	hasBody := false
	for _, route := range routes {
		for _, param := range route.Params {
			if param.Source == "body" {
				hasBody = true
				continue
			}
			name := flagName(param.Name)
			if cmd.Flags().Lookup(name) != nil {
				continue
			}
			paramFlags = append(paramFlags, paramFlag{flag: name, param: param.Name})
			cmd.Flags().String(name, "", "Request "+param.Source+" parameter "+param.Name)
			if enforceRequired && param.Required {
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
	best := routes[0]
	bestSatisfied, bestTotal := pathParamCount(best, values)
	for _, route := range routes[1:] {
		satisfied, total := pathParamCount(route, values)
		if satisfied > bestSatisfied || (satisfied == bestSatisfied && total < bestTotal) {
			best = route
			bestSatisfied = satisfied
			bestTotal = total
		}
	}
	return best
}

func pathParamCount(route router.RouteInfo, values map[string]string) (int, int) {
	satisfied := 0
	total := 0
	for _, param := range route.Params {
		if param.Source != "path" {
			continue
		}
		total++
		if values[param.Name] != "" {
			satisfied++
		}
	}
	return satisfied, total
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
