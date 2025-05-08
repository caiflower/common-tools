package dbv1

import (
	"github.com/caiflower/common-tools/pkg/logger"
	"github.com/uptrace/bun"
)

type BatchTx struct {
	tx   *bun.Tx
	conn *bun.DB
	txs  []func(tx *bun.Tx) error
}

func NewBatchTx(conn bun.IDB) *BatchTx {
	bt := &BatchTx{
		txs: make([]func(tx *bun.Tx) error, 0, 5),
	}
	if con, ok := conn.(*bun.DB); ok {
		bt.conn = con
	} else if tx, ok2 := conn.(*bun.Tx); ok2 {
		bt.tx = tx
	}
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

	// 默认使用外部传进来的事务
	tx, isMyTx := b.tx, false

	// 如果未指定事务，则创建新的事务
	if tx == nil {
		newTx, txErr := b.conn.Begin()
		if txErr != nil {
			return txErr
		}
		tx, isMyTx = &newTx, true
	}

	// 执行
	for _, fc := range b.txs {
		if err = fc(tx); err != nil {
			break
		}
	}

	// 如果是内部创建的事务，退出前需要结束事务
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
