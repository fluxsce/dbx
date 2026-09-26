package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Begin starts a transaction at the engine default isolation level.
// The caller must Commit or Rollback the returned session.
// Equivalent to BeginOptions(ctx, nil).
func (d *DB) Begin(ctx context.Context) (*DB, error) {
	return d.BeginOptions(ctx, nil)
}

// BeginOptions starts a transaction with opt. A nil opt uses the engine default.
// The caller must Commit or Rollback. The returned *DB is one goroutine only,
// and must not be used after Commit or Rollback.
// Begin on a session that is already a transaction returns an error.
// Use Tx to run more work inside the current transaction.
func (d *DB) BeginOptions(ctx context.Context, opt *sql.TxOptions) (*DB, error) {
	if d == nil || d.pool == nil {
		return nil, fmt.Errorf("dbx: nil session")
	}
	if d.done {
		return nil, fmt.Errorf("dbx: transaction already finished")
	}
	if d.tx != nil {
		return nil, fmt.Errorf("dbx: transaction already started")
	}
	start := d.mark()
	ctx = d.before(ctx, "begin", "")
	raw, err := d.pool.BeginTx(ctx, opt)
	d.finish(ctx, "begin", "", nil, 0, err, start)
	if err != nil {
		return nil, err
	}
	return &DB{pool: d.pool, tx: raw, dial: d.dial, hooks: d.hooks, trace: d.trace}, nil
}

// Commit commits this transaction. Further calls on this session fail.
// Commit on a pool session returns an error.
func (d *DB) Commit() error {
	if d == nil || d.tx == nil {
		return fmt.Errorf("dbx: not a transaction")
	}
	if d.done {
		return fmt.Errorf("dbx: transaction already finished")
	}
	d.done = true
	start := d.mark()
	err := d.tx.Commit()
	d.finish(context.Background(), "commit", "", nil, 0, err, start)
	return err
}

// Rollback rolls back this transaction. Further calls on this session fail.
// Rollback on a pool session returns an error.
func (d *DB) Rollback() error {
	if d == nil || d.tx == nil {
		return fmt.Errorf("dbx: not a transaction")
	}
	if d.done {
		return fmt.Errorf("dbx: transaction already finished")
	}
	d.done = true
	start := d.mark()
	err := d.tx.Rollback()
	d.finish(context.Background(), "rollback", "", nil, 0, err, start)
	return err
}

// Tx starts a transaction at the engine default isolation level.
// Equivalent to TxOptions(ctx, nil, fn).
func (d *DB) Tx(ctx context.Context, fn func(*DB) error) error {
	return d.TxOptions(ctx, nil, fn)
}

// TxOptions starts a transaction with opt, calls fn, then commits.
// A nil opt uses the engine default isolation level.
// A non-nil error or a panic rolls the transaction back.
// If this session is already a transaction, fn joins it: opt is ignored,
// and Commit or Rollback stays with the outer caller.
// The *DB passed to fn must stay on this goroutine and must not be used
// after fn returns.
func (d *DB) TxOptions(ctx context.Context, opt *sql.TxOptions, fn func(*DB) error) error {
	if fn == nil {
		return fmt.Errorf("dbx: nil transaction function")
	}
	if d != nil && d.tx != nil {
		if d.done {
			return fmt.Errorf("dbx: transaction already finished")
		}
		return fn(d)
	}
	tx, err := d.BeginOptions(ctx, opt)
	if err != nil {
		return err
	}
	defer func() {
		if !tx.done {
			_ = tx.Rollback() // 回调出错或 panic 时回滚；Commit 成功后 done 已置位，不会再滚
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
