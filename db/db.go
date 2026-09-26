// Package db is the call surface of github.com/fluxsce/dbx.
//
// Callers write SQL with @name placeholders and page with LimitSQL. Engine
// differences stay in Dialect plugins. Blank-import a driver package to register it.
//
// *DB is both the pool session and a transaction session. Exec, Query, Insert,
// Update, Delete, Get, Select, and Each are the same methods on either.
//
// A pool session auto-commits each successful statement. Begin returns a
// transaction the caller must Commit or Rollback. Tx commits when the callback
// returns nil and rolls back otherwise.
//
// A pool *DB may be used from many goroutines. A transaction *DB may be used
// from only one goroutine, and only until Commit or Rollback.
//
// Bind variables are @name only. The key in Args has no @. Do not write ? or $1
// in SQL. Optional column types live in github.com/fluxsce/dbx/record.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Config 打开连接的配置。部署侧改 Driver / DSN 与连接池，代码无引擎分支。
// 池参数与 database/sql 同名；0 表示沿用标准库默认（MaxOpen 不限制、MaxIdle=2、无生命周期）。
type Config struct {
	// Driver 为引擎名或别名，如 sqlite、postgres、pg、mysql。
	Driver string
	// DSN 为该引擎的连接串；SQLite 一般为文件路径。
	DSN string
	// MaxOpenConns 为连接池上限；0 表示不限制（database/sql 默认）。
	MaxOpenConns int
	// MaxIdleConns 为空闲连接数；0 表示沿用标准库默认（通常为 2）。
	MaxIdleConns int
	// ConnMaxLifetime 为连接最长存活时间；0 表示不回收。MySQL 建议小于服务端 wait_timeout。
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime 为空闲连接最长存活时间；0 表示不因空闲关闭。
	ConnMaxIdleTime time.Duration
	// Trace 在每条语句和事务提交/回滚之后调用。nil 表示不记录。
	// 慢查询阈值、SQL 脱敏和日志格式由调用方决定。
	Trace TraceFunc
}

// ctxExecer 是 *sql.DB 与 *sql.Tx 的公共执行面，避免为事务再抄一套方法。
type ctxExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DB is the only session type. A nil tx auto-commits each statement.
// A non-nil tx is bound to that transaction until Commit or Rollback.
type DB struct {
	pool  *sql.DB   // pool; Close shuts this down
	tx    *sql.Tx   // non-nil while this session is a transaction
	dial  Dialect   // bound at Open, read-only afterwards
	done  bool      // Commit or Rollback already finished this transaction
	trace TraceFunc // copied onto each Begin; nil disables tracing
}

// Open 按 Driver 查找已 Register 的方言与 opener，Ping 成功后返回连接池会话。
// 未 blank-import 对应 driver 包时返回“not registered”。
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if strings.TrimSpace(cfg.Driver) == "" {
		return nil, fmt.Errorf("dbx: driver is required")
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("dbx: dsn is required")
	}
	d, open, err := lookup(cfg.Driver)
	if err != nil {
		return nil, err
	}
	raw, err := open(cfg.DSN)
	if err != nil {
		if raw != nil {
			_ = raw.Close() // open 失败但已拿出池时也要关掉
		}
		return nil, fmt.Errorf("dbx: open %s: %w", NormalizeDriver(cfg.Driver), err)
	}
	applyPool(raw, cfg)
	if err := raw.PingContext(ctx); err != nil {
		_ = raw.Close() // Ping 失败时关掉半开连接，避免泄漏
		return nil, fmt.Errorf("dbx: ping: %w", err)
	}
	return &DB{pool: raw, dial: d, trace: cfg.Trace}, nil
}

// applyPool 把 Config 中大于 0 的池参数写到 *sql.DB；0 表示不改标准库默认。
func applyPool(raw *sql.DB, cfg Config) {
	if cfg.MaxOpenConns > 0 {
		raw.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		raw.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		raw.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		raw.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}

// conn returns the executor: the open transaction, or the pool when auto-committing.
func (d *DB) conn() (ctxExecer, error) {
	if d == nil || d.pool == nil {
		return nil, fmt.Errorf("dbx: nil session")
	}
	if d.done {
		return nil, fmt.Errorf("dbx: transaction already finished")
	}
	if d.tx != nil {
		return d.tx, nil
	}
	return d.pool, nil
}

// InTx reports whether this session is an open transaction.
func (d *DB) InTx() bool { return d != nil && d.tx != nil && !d.done }

// Driver 返回规范化引擎名（如 postgres）。业务 SQL 不应按此名分支。
func (d *DB) Driver() string {
	if d == nil || d.dial == nil {
		return ""
	}
	return d.dial.Name()
}

// SQL 返回底层连接池，仅迁移脚本或驱动特例使用。
func (d *DB) SQL() *sql.DB {
	if d == nil {
		return nil
	}
	return d.pool
}

// QuoteIdent 按当前引擎引用标识符，供拼接表名与列名。
func (d *DB) QuoteIdent(name string) string { return d.dial.QuoteIdent(name) }

// Close 关闭连接池。事务态会话不要调用 Close，由根会话在进程退出时关。
func (d *DB) Close() error {
	if d == nil || d.pool == nil || d.tx != nil {
		return nil
	}
	return d.pool.Close()
}

// Exec runs a statement that does not return rows (INSERT, UPDATE, DELETE, DDL).
// On a pool session the statement auto-commits when it succeeds. query uses @name.
func (d *DB) Exec(ctx context.Context, query string, args Args) (sql.Result, error) {
	start := d.mark()
	q, vals, err := compile(d.dial, query, args)
	if err != nil {
		d.finish(ctx, "exec", query, nil, 0, err, start)
		return nil, err
	}
	ex, err := d.conn()
	if err != nil {
		d.finish(ctx, "exec", q, vals, 0, err, start)
		return nil, err
	}
	res, err := ex.ExecContext(ctx, q, vals...)
	var n int64
	if err == nil && res != nil {
		n, _ = res.RowsAffected()
	}
	d.finish(ctx, "exec", q, vals, n, err, start)
	return res, err
}

// execBound 执行已展开为 ? 的语句，再按当前引擎 Rebind（Postgres 变成 $n）。
// 供 Insert 批量 VALUES、Update 分绑 SET/WHERE 使用，不走业务 SQL 的 @name 编译。
func (d *DB) execBound(ctx context.Context, qmark string, vals []any) (sql.Result, error) {
	start := d.mark()
	q := d.dial.Rebind(qmark)
	ex, err := d.conn()
	if err != nil {
		d.finish(ctx, "exec", q, vals, 0, err, start)
		return nil, err
	}
	res, err := ex.ExecContext(ctx, q, vals...)
	var n int64
	if err == nil && res != nil {
		n, _ = res.RowsAffected()
	}
	d.finish(ctx, "exec", q, vals, n, err, start)
	return res, err
}

// Query 执行读语句，返回多行。调用方负责关闭 Rows。
func (d *DB) Query(ctx context.Context, query string, args Args) (*sql.Rows, error) {
	start := d.mark()
	q, vals, err := compile(d.dial, query, args)
	if err != nil {
		d.finish(ctx, "query", query, nil, 0, err, start)
		return nil, err
	}
	ex, err := d.conn()
	if err != nil {
		d.finish(ctx, "query", q, vals, 0, err, start)
		return nil, err
	}
	rows, err := ex.QueryContext(ctx, q, vals...)
	d.finish(ctx, "query", q, vals, 0, err, start)
	return rows, err
}

// QueryRow 执行最多一行。compile 失败时返回 error；无行时由 Scan 给出 sql.ErrNoRows。
func (d *DB) QueryRow(ctx context.Context, query string, args Args) (*sql.Row, error) {
	start := d.mark()
	q, vals, err := compile(d.dial, query, args)
	if err != nil {
		d.finish(ctx, "query", query, nil, 0, err, start)
		return nil, err
	}
	ex, err := d.conn()
	if err != nil {
		d.finish(ctx, "query", q, vals, 0, err, start)
		return nil, err
	}
	// QueryRowContext 推迟到 Scan 才访问驱动，这里不记成功事件。
	return ex.QueryRowContext(ctx, q, vals...), nil
}

// QueryPage 先跑 countSQL 得到 total，再按当前引擎给 listSQL 分页。
// countSQL 与 listSQL 应使用同一套 @name。后缀引擎不要在 listSQL 里自行写 LIMIT / FETCH。
// Oracle 11g 由 PageSQL 改写整句。返回的 Rows 由调用方关闭。
func (d *DB) QueryPage(ctx context.Context, countSQL, listSQL string, page Page, args Args) (rows *sql.Rows, total int64, err error) {
	paged, err := d.PageSQL(listSQL, page)
	if err != nil {
		return nil, 0, err
	}
	row, err := d.QueryRow(ctx, countSQL, args)
	if err != nil {
		return nil, 0, err
	}
	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}
	args = BindPage(args, page)
	rows, err = d.Query(ctx, paged, args)
	return rows, total, err
}
