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

package sqld

import (
	"context"

	dbv1 "github.com/caiflower/common-tools/db/v1"
	"github.com/caiflower/common-tools/taskx/dao"
)

// sqlStore implements dao.Store using the bun ORM transaction model.
type sqlStore struct {
	client *dbv1.Client
}

// NewStore creates a dao.Store backed by a SQL database via dbv1.Client.
func NewStore(client *dbv1.Client) dao.Store {
	return &sqlStore{client: client}
}

// RunInTx opens a bun.Tx, invokes fn with the tx stored in context, and commits
// on nil error or rolls back otherwise. If the context already carries a *bun.Tx
// (via dao.WithTxContext), fn runs within that existing transaction.
func (s *sqlStore) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// If already inside a transaction, reuse it
	if existingTx := dao.TxFromContext(ctx); existingTx != nil {
		return fn(ctx)
	}

	tx, cancel, err := s.client.Begin(ctx)
	if err != nil {
		return err
	}
	defer cancel()

	txCtx := dao.WithTxContext(ctx, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Client returns the underlying dbv1.Client.
func (s *sqlStore) Client() *dbv1.Client {
	return s.client
}
