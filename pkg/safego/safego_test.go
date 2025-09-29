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

 package safego

import (
	"context"
	"sync"
	"testing"

	golocalv1 "github.com/caiflower/common-tools/pkg/golocal/v1"
	"github.com/stretchr/testify/assert"
)

func TestSageGo(t *testing.T) {
	ctx := context.Background()
	trace := "test"
	golocalv1.PutTraceID(trace)
	golocalv1.PutContext(ctx)

	wg := &sync.WaitGroup{}
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		Go(func() {
			defer wg.Done()
			assert.Equal(t, golocalv1.GetTraceID(), trace)
			assert.Equal(t, golocalv1.GetContext(), ctx)
		})
	}
	wg.Wait()
}
