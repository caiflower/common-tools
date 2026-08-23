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

package tools

import (
	"os"
	"strings"
)

// ResolvePasswordFromEnv returns the password from environment variables when
// present. Per-instance variable <component>_PASSWORD_<name> wins over the
// generic <component>_PASSWORD; otherwise fallback is returned.
func ResolvePasswordFromEnv(component, name, fallback string) string {
	if v, ok := os.LookupEnv(envKey(component, name)); ok {
		return v
	}
	if v, ok := os.LookupEnv(envKey(component, "")); ok {
		return v
	}
	return fallback
}

func envKey(component, name string) string {
	component = normalizeEnvPart(component)
	name = normalizeEnvPart(name)
	if name == "" {
		return component + "_PASSWORD"
	}
	return component + "_PASSWORD_" + name
}

func normalizeEnvPart(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
