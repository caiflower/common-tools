/*
 * Copyright 2022 CloudWeGo Authors
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

package discovery

import (
	"context"
	"testing"

	"github.com/caiflower/common-tools/web/app/server/registry"
	"github.com/stretchr/testify/assert"
)

func TestInstance(t *testing.T) {
	network := "192.168.1.1"
	address := "/hello"
	weight := 1
	instance := NewInstance(network, address, weight, nil)

	assert.Equal(t, network, instance.Address().Network())
	assert.Equal(t, address, instance.Address().String())
	assert.Equal(t, weight, instance.Weight())
	val, ok := instance.Tag("name")
	assert.Equal(t, "", val)
	assert.False(t, ok)

	instance2 := NewInstance("", "", 0, nil)
	assert.Equal(t, registry.DefaultWeight, instance2.Weight())
}

func TestSynthesizedResolver(t *testing.T) {
	targetFunc := func(ctx context.Context, target *TargetInfo) string {
		return "hello"
	}
	resolveFunc := func(ctx context.Context, key string) (Result, error) {
		return Result{CacheKey: "name"}, nil
	}
	nameFunc := func() string {
		return "raymonder"
	}
	resolver := SynthesizedResolver{
		TargetFunc:  targetFunc,
		ResolveFunc: resolveFunc,
		NameFunc:    nameFunc,
	}

	assert.Equal(t, "hello", resolver.Target(context.Background(), &TargetInfo{}))
	res, err := resolver.Resolve(context.Background(), "")
	assert.Equal(t, "name", res.CacheKey)
	assert.Nil(t, err)
	assert.Equal(t, "raymonder", resolver.Name())

	resolver2 := SynthesizedResolver{
		TargetFunc:  nil,
		ResolveFunc: nil,
		NameFunc:    nil,
	}
	assert.Equal(t, "", resolver2.Target(context.Background(), &TargetInfo{}))
	assert.Equal(t, "", resolver2.Name())
}
