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
	"testing"
)

func TestResolvePasswordFromEnv(t *testing.T) {
	t.Setenv("DB_PASSWORD", "generic")
	t.Setenv("DB_PASSWORD_USER_01", "per-instance")

	if got := ResolvePasswordFromEnv("db", "", "config"); got != "generic" {
		t.Fatalf("generic env should win over config, got %q", got)
	}
	if got := ResolvePasswordFromEnv("db", "user-01", "config"); got != "per-instance" {
		t.Fatalf("per-instance env should win over generic env, got %q", got)
	}
}

func TestResolvePasswordFromEnvFallback(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")

	if got := ResolvePasswordFromEnv("db", "", "config"); got != "" {
		t.Fatalf("empty env value should override config, got %q", got)
	}
	if got := ResolvePasswordFromEnv("redis", "", "config"); got != "config" {
		t.Fatalf("missing env should fall back to config, got %q", got)
	}
}
