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
	"fmt"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/uptrace/bun"
)

type BatchTx struct {
	tx      *bun.Tx
	conn    *bun.DB
	timeout time.Duration
	ctx     context.Context
	txs     []func(tx *bun.Tx) error
}

func NewBatchTx(conn bun.IDB, timeout ...time.Duration) *BatchTx {
	if conn == nil {
		panic("BatchTx: conn must not be nil")
	}
	bt := &BatchTx{
		txs: make([]func(tx *bun.Tx) error, 0, 5),
	}
	if len(timeout) > 0 {
		bt.timeout = timeout[0]
	} else {
		bt.timeout = 30 * time.Second
	}
	if con, ok := conn.(*bun.DB); ok {
		bt.conn = con
	} else if tx, ok2 := conn.(*bun.Tx); ok2 {
		bt.tx = tx
	}
	return bt
}

func NewBatchTxWithContext(ctx context.Context, conn bun.IDB, timeout ...time.Duration) *BatchTx {
	bt := NewBatchTx(conn, timeout...)
	bt.ctx = ctx
	return bt
}

func (b *BatchTx) Add(txFunc func(tx *bun.Tx) error) *BatchTx {
	if txFunc == nil {
		return b
	}
	b.txs = append(b.txs, txFunc)
	return b
}

func (b *BatchTx) Submit() (err error) {

	if len(b.txs) == 0 {
		return nil
	}

	if b.tx == nil && b.conn == nil {
		return fmt.Errorf("BatchTx: no database connection or transaction provided")
	}

	tx, isMyTx := b.tx, false
	var cancel context.CancelFunc

	if tx == nil {
		parentCtx := b.ctx
		if parentCtx == nil {
			parentCtx = context.Background()
		}
		ctx, c := context.WithTimeout(parentCtx, b.timeout)
		cancel = c
		defer cancel()
		newTx, txErr := b.conn.BeginTx(ctx, nil)
		if txErr != nil {
			cancel()
			return txErr
		}
		tx, isMyTx = &newTx, true
	}

	defer func() {
		if r := recover(); r != nil {
			if isMyTx {
				if _err := tx.Rollback(); _err != nil {
					logger.Warn("rollback failed after panic. err: %v", _err.Error())
				}
			}
			panic(r)
		}
	}()

	for _, fc := range b.txs {
		if b.ctx != nil {
			select {
			case <-b.ctx.Done():
				err = b.ctx.Err()
				break
			default:
			}
			if err != nil {
				break
			}
		}
		if err = fc(tx); err != nil {
			break
		}
	}

	if isMyTx {
		if err != nil {
			if _err := tx.Rollback(); _err != nil {
				logger.Warn("rollback failed. err: %v", _err.Error())
			}
		} else {
			err = tx.Commit()
		}
	}

	return
}
