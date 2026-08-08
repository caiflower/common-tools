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

package router

import (
	"context"
	"reflect"
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/web/protocol/consts"
	"github.com/caiflower/common-tools/web/router/method"
)

// ParamInfo describes one request parameter for CLI command generation.
type ParamInfo struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Verf     string `json:"verf,omitempty"`
}

// RouteInfo describes one registered route for CLI command generation.
type RouteInfo struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	OperationID string      `json:"operationID"`
	Resource    string      `json:"resource"`
	Verb        string      `json:"verb"`
	Params      []ParamInfo `json:"params,omitempty"`
	Static      bool        `json:"static,omitempty"`
}

func deriveVerb(httpMethod string) string {
	switch strings.ToUpper(httpMethod) {
	case consts.MethodGet:
		return "get"
	case consts.MethodPost:
		return "create"
	case consts.MethodPut:
		return "update"
	case consts.MethodPatch:
		return "patch"
	case consts.MethodDelete:
		return "delete"
	default:
		return "call"
	}
}

func deriveResourceFromName(name string) string {
	verbs := []string{"get", "create", "update", "patch", "delete", "list", "search"}
	for _, verb := range verbs {
		if len(name) <= len(verb) {
			continue
		}
		if !strings.EqualFold(name[:len(verb)], verb) {
			continue
		}
		rest := name[len(verb):]
		if rest[0] < 'A' || rest[0] > 'Z' {
			continue
		}
		return lowerFirst(rest)
	}
	return ""
}

func deriveResourceFromPath(path string) string {
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		segment := parts[i]
		if segment == "" || strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
			continue
		}
		return segment
	}
	return ""
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func simpleName(name string) string {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func extractParams(m *method.Method, httpMethod string) []ParamInfo {
	if m == nil || !m.HasArgs() {
		return nil
	}
	target := m.GetTargetMethod()
	if target == nil || !target.HasArgs() {
		return nil
	}
	argIndex := requestArgIndex(m, target)
	if argIndex < 0 {
		return nil
	}
	return extractParamsFromArg(target.GetArgInfo(argIndex), httpMethod, "")
}

func requestArgIndex(m *method.Method, target *basic.Method) int {
	args := target.GetArgs()
	if len(args) == 0 {
		return -1
	}
	if len(args) > 1 {
		return len(args) - 1
	}
	if args[0].Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		return -1
	}
	return 0
}

func extractParamsFromArg(arg *basic.ArgInfo, httpMethod string, prefix string) []ParamInfo {
	if arg == nil {
		return nil
	}
	t := arg.Type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	return extractParamsFromType(t, httpMethod, prefix, make(map[reflect.Type]bool))
}

func extractParamsFromType(t reflect.Type, httpMethod, prefix string, seen map[reflect.Type]bool) []ParamInfo {
	var params []ParamInfo
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		source, name := paramSourceAndName(field, httpMethod)
		if source != "" {
			verf := strings.TrimSpace(field.Tag.Get("verf"))
			params = append(params, ParamInfo{
				Name:     name,
				Source:   source,
				Type:     field.Type.String(),
				Required: requiredFromVerf(verf),
				Verf:     verf,
			})
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && !seen[fieldType] {
			seen[fieldType] = true
			params = append(params, extractParamsFromType(fieldType, httpMethod, name+".", seen)...)
		}
	}
	return params
}

func paramSourceAndName(field reflect.StructField, httpMethod string) (string, string) {
	if value := strings.TrimSpace(field.Tag.Get("path")); value != "" {
		return "path", value
	}
	if value := strings.TrimSpace(field.Tag.Get("query")); value != "" {
		return "query", value
	}
	if value := strings.TrimSpace(field.Tag.Get("header")); value != "" {
		return "header", value
	}
	jsonName := strings.TrimSpace(field.Tag.Get("json"))
	if jsonName == "" {
		jsonName = field.Name
	}
	if strings.EqualFold(httpMethod, consts.MethodGet) || strings.EqualFold(httpMethod, consts.MethodHead) {
		return "query", jsonName
	}
	return "body", jsonName
}

func requiredFromVerf(verf string) bool {
	return strings.Contains(verf, "required")
}

func (h *Handler) recordRoute(httpMethod string, path string, handlers HandlersChain) {
	if len(handlers) == 0 {
		return
	}
	target := handlers[len(handlers)-1]
	operationID := simpleName(target.GetAction())
	if operationID == "" {
		operationID = httpMethod + "_" + strings.ReplaceAll(strings.Trim(path, "/"), "/", "_")
	}

	var params []ParamInfo
	if target.GetType() == method.DefaultTypeOfMethod || target.GetType() == method.GrpcTypeOfMethod {
		params = filterPathParams(extractParams(&target, httpMethod), path)
	}

	resource := deriveResourceFromName(operationID)
	if resource == "" {
		resource = deriveResourceFromPath(path)
	}
	verb := deriveVerb(httpMethod)

	info := RouteInfo{
		Method:      httpMethod,
		Path:        path,
		OperationID: operationID,
		Resource:    resource,
		Verb:        verb,
		Params:      params,
	}

	key := httpMethod + " " + path
	h.routeMu.Lock()
	if override, ok := h.cliOverrides[key]; ok {
		info.Resource = override.resource
		info.Verb = override.verb
	}
	h.routeInfos = append(h.routeInfos, info)
	h.routeMu.Unlock()
}

func filterPathParams(params []ParamInfo, path string) []ParamInfo {
	filtered := make([]ParamInfo, 0, len(params))
	for _, param := range params {
		if param.Source == "path" && !pathHasParam(path, param.Name) {
			continue
		}
		filtered = append(filtered, param)
	}
	return filtered
}

func pathHasParam(path, name string) bool {
	for _, segment := range strings.Split(path, "/") {
		if strings.HasPrefix(segment, ":") && segment[1:] == name {
			return true
		}
		if strings.HasPrefix(segment, "*") && segment[1:] == name {
			return true
		}
	}
	return false
}

// Routes returns a snapshot of all route metadata.
func (h *Handler) Routes() []RouteInfo {
	h.routeMu.RLock()
	defer h.routeMu.RUnlock()
	routes := make([]RouteInfo, len(h.routeInfos))
	copy(routes, h.routeInfos)
	return routes
}

// CLIRoute overrides resource and verb for a route.
func (h *Handler) CLIRoute(methodName, path, resource, verb string) {
	key := methodName + " " + path
	h.routeMu.Lock()
	if h.cliOverrides == nil {
		h.cliOverrides = make(map[string]cliOverride)
	}
	h.cliOverrides[key] = cliOverride{resource: resource, verb: verb}
	for i := range h.routeInfos {
		if h.routeInfos[i].Method == methodName && h.routeInfos[i].Path == path {
			h.routeInfos[i].Resource = resource
			h.routeInfos[i].Verb = verb
			break
		}
	}
	h.routeMu.Unlock()
}

type cliOverride struct {
	resource string
	verb     string
}
