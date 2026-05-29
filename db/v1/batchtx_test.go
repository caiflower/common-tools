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

package dbv1

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

type testItem struct {
	ID   int64  `bun:"id,pk,autoincrement"`
	Name string `bun:"name"`
}

func newTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewCreateTable().Model((*testItem)(nil)).Exec(context.Background())
	require.NoError(t, err)
	return db
}

func TestBatchTx_Submit_NilConn(t *testing.T) {
	require.Panics(t, func() {
		NewBatchTx(nil)
	})
}

func TestBatchTx_Submit_UnsupportedIDB(t *testing.T) {
	type fakeIDB struct {
		bun.IDB
	}
	bt := NewBatchTx(fakeIDB{})
	bt.Add(func(tx *bun.Tx) error {
		return nil
	})
	err := bt.Submit()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no database connection or transaction provided")
}

func TestBatchTx_Submit_EmptyTxs(t *testing.T) {
	db := newTestDB(t)
	bt := NewBatchTx(db)
	err := bt.Submit()
	assert.NoError(t, err)
}

func TestBatchTx_Add_Nil(t *testing.T) {
	db := newTestDB(t)
	bt := NewBatchTx(db)
	result := bt.Add(nil)
	assert.Equal(t, bt, result)
	assert.Empty(t, bt.txs)
}

func TestBatchTx_Submit_WithConn_Commit(t *testing.T) {
	db := newTestDB(t)
	bt := NewBatchTx(db)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "item1"}).Exec(context.Background())
		return err
	})
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "item2"}).Exec(context.Background())
		return err
	})
	err := bt.Submit()
	assert.NoError(t, err)

	var items []testItem
	err = db.NewSelect().Model(&items).Order("id asc").Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "item1", items[0].Name)
	assert.Equal(t, "item2", items[1].Name)
}

func TestBatchTx_Submit_WithConn_Rollback(t *testing.T) {
	db := newTestDB(t)
	bt := NewBatchTx(db)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "item1"}).Exec(context.Background())
		return err
	})
	bt.Add(func(tx *bun.Tx) error {
		return fmt.Errorf("intentional error")
	})
	err := bt.Submit()
	assert.Error(t, err)
	assert.Equal(t, "intentional error", err.Error())

	count, err := db.NewSelect().Model((*testItem)(nil)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestBatchTx_Submit_WithExternalTx(t *testing.T) {
	db := newTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	bt := NewBatchTx(&tx)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "ext_item"}).Exec(context.Background())
		return err
	})
	err = bt.Submit()
	assert.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	var items []testItem
	err = db.NewSelect().Model(&items).Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "ext_item", items[0].Name)
}

func TestBatchTx_Submit_WithExternalTx_RollbackByExternal(t *testing.T) {
	db := newTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	bt := NewBatchTx(&tx)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "ext_item_rollback"}).Exec(context.Background())
		return err
	})
	err = bt.Submit()
	assert.NoError(t, err)

	err = tx.Rollback()
	require.NoError(t, err)

	count, err := db.NewSelect().Model((*testItem)(nil)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestBatchTx_TimeoutParameter(t *testing.T) {
	db := newTestDB(t)
	customTimeout := 5 * time.Second
	bt := NewBatchTx(db, customTimeout)
	assert.Equal(t, customTimeout, bt.timeout)
}

func TestBatchTx_DefaultTimeout(t *testing.T) {
	db := newTestDB(t)
	bt := NewBatchTx(db)
	assert.Equal(t, 30*time.Second, bt.timeout)
}

func TestBatchTx_Submit_PanicRecovery(t *testing.T) {
	db := newTestDB(t)
	bt := NewBatchTx(db)
	bt.Add(func(tx *bun.Tx) error {
		panic("test panic")
	})
	assert.Panics(t, func() {
		_ = bt.Submit()
	})

	count, err := db.NewSelect().Model((*testItem)(nil)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestBatchTxWithContext_SetsCtx(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	bt := NewBatchTxWithContext(ctx, db)
	assert.Equal(t, ctx, bt.ctx)
	assert.Equal(t, 30*time.Second, bt.timeout)
}

func TestBatchTxWithContext_CustomTimeout(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	customTimeout := 10 * time.Second
	bt := NewBatchTxWithContext(ctx, db, customTimeout)
	assert.Equal(t, ctx, bt.ctx)
	assert.Equal(t, customTimeout, bt.timeout)
}

func TestBatchTxWithContext_CancelledCtx(t *testing.T) {
	db := newTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bt := NewBatchTxWithContext(ctx, db)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "should_not_insert"}).Exec(context.Background())
		return err
	})
	err := bt.Submit()
	assert.ErrorIs(t, err, context.Canceled)

	count, _ := db.NewSelect().Model((*testItem)(nil)).Count(context.Background())
	assert.Equal(t, 0, count)
}

func TestBatchTxWithContext_CancelDuringExecution(t *testing.T) {
	db := newTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())

	bt := NewBatchTxWithContext(ctx, db)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "first"}).Exec(context.Background())
		return err
	})
	bt.Add(func(tx *bun.Tx) error {
		cancel()
		return nil
	})
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "third"}).Exec(context.Background())
		return err
	})
	err := bt.Submit()
	assert.ErrorIs(t, err, context.Canceled)

	count, _ := db.NewSelect().Model((*testItem)(nil)).Count(context.Background())
	assert.Equal(t, 0, count)
}

func TestBatchTxWithContext_CancelledCtxWithExternalTx(t *testing.T) {
	db := newTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bt := NewBatchTxWithContext(ctx, &tx)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "should_not_insert"}).Exec(context.Background())
		return err
	})
	err = bt.Submit()
	assert.ErrorIs(t, err, context.Canceled)

	tx.Rollback()
}

func TestBatchTxWithContext_Commit(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	bt := NewBatchTxWithContext(ctx, db)
	bt.Add(func(tx *bun.Tx) error {
		_, err := tx.NewInsert().Model(&testItem{Name: "ctx_item"}).Exec(context.Background())
		return err
	})
	err := bt.Submit()
	assert.NoError(t, err)

	var items []testItem
	err = db.NewSelect().Model(&items).Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "ctx_item", items[0].Name)
}
