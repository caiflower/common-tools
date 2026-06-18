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

package dao

import "context"

// Store is the storage-backend-agnostic abstraction for transaction management.
//
// Each DAO implementation holds a reference to its own Store. Callers that need
// to coordinate multiple DAO writes atomically use RunInTx instead of passing
// *bun.Tx through every DAO method.
//
//   - SQL backend: RunInTx opens a bun.Tx, commits on nil return, rolls back on error.
//   - Redis backend: RunInTx executes the callback within a Lua script or pipeline
//     to guarantee atomicity.
type Store interface {
	// RunInTx executes fn within a single transaction boundary.
	// If fn returns nil the transaction is committed; otherwise it is rolled back.
	// If the context already carries a transaction (via WithTxContext), fn runs
	// within that existing transaction without creating a new one.
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// txContextKey is the context key for storing an active transaction.
type txContextKey struct{}

// WithTxContext stores a transaction object in the context.
// DAO implementations retrieve it via TxFromContext to reuse an existing
// transaction started by RunInTx.
func WithTxContext(ctx context.Context, tx interface{}) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext retrieves a transaction object from the context.
// Returns nil if no transaction is present.
func TxFromContext(ctx context.Context) interface{} {
	return ctx.Value(txContextKey{})
}
