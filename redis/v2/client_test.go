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

package v2

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	xredis "github.com/caiflower/common-tools/redis"
	"github.com/redis/go-redis/v9"
	trequire "github.com/stretchr/testify/require"
)

func TestNewRedisClient_DisableIdentity(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "standalone", mode: ""},
		{name: "cluster", mode: xredis.ClusterMode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			client, err := NewRedisClient(Config{
				Addrs:           []string{mr.Addr()},
				Mode:            tt.mode,
				DisableIdentity: true,
				EnableMetrics:   "false",
			})
			trequire.NoError(t, err)
			defer client.Close()

			switch c := client.GetRedis().(type) {
			case *redis.Client:
				trequire.True(t, c.Options().DisableIdentity)
			case *redis.ClusterClient:
				trequire.True(t, c.Options().DisableIdentity)
			default:
				t.Fatalf("unexpected redis client type: %T", client.GetRedis())
			}
		})
	}
}
