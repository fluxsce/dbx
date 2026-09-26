package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

const (
	defaultInsertRows   = 200  // 未声明上限的引擎，单批行数
	defaultInsertParams = 1000 // 未声明上限的引擎，单批占位符
)

// normalizedInsertLimit 补上未实现 InsertDialect 时的默认值。
// 已实现时，小于 1 的行数和占位符数仍用默认；Atomic 与 Prepare 以驱动返回的为准。
func normalizedInsertLimit(dial Dialect) InsertLimit {
	lim := InsertLimit{MaxRows: defaultInsertRows, MaxParams: defaultInsertParams, Atomic: true, Prepare: true}
	if dial == nil {
		return lim
	}
	custom, ok := dial.(InsertDialect)
	if !ok {
		return lim
	}
	got := custom.InsertLimit()
	if got.MaxRows > 0 {
		lim.MaxRows = got.MaxRows
	}
	if got.MaxParams > 0 {
		lim.MaxParams = got.MaxParams
	}
	lim.Atomic = got.Atomic
	lim.Prepare = got.Prepare
	return lim
}

// chunkRows 按列数收束单批行数，保证占位符不超过引擎上限。
func chunkRows(lim InsertLimit, cols int) int {
	if cols < 1 {
		cols = 1
	}
	maxRows, maxParams := lim.MaxRows, lim.MaxParams
	if maxRows < 1 {
		maxRows = defaultInsertRows
	}
	if maxParams < 1 {
		maxParams = defaultInsertParams
	}
	byParams := maxParams / cols
	if byParams < 1 {
		byParams = 1
	}
	if byParams < maxRows {
		return byParams
	}
	return maxRows
}

// insertSQL 把多行拆成若干条多值 INSERT。
// 超过一批且引擎能回滚时，在连接池上包进一个事务，避免前一批已提交、后一批失败。
// 已处于事务中时直接写在当前事务上，不再另开事务，也不会再向池申请连接。
func (d *DB) insertSQL(ctx context.Context, table string, names []string, byName map[string]colField, list []reflect.Value) error {
	lim := normalizedInsertLimit(d.dial)
	size := chunkRows(lim, len(names))
	if len(list) > size && lim.Atomic && !d.InTx() {
		return d.Tx(ctx, func(tx *DB) error {
			return tx.writeSQLChunks(ctx, table, names, byName, list, size, lim.Prepare)
		})
	}
	return d.writeSQLChunks(ctx, table, names, byName, list, size, lim.Prepare)
}

// writeSQLChunks 按 size 分批执行。满批语句文本相同，两批及以上时预编译一次，函数返回前关闭。
// 预编译挂在当前会话上：事务里用 *sql.Tx，避免池上再取一条连接造成死锁。
func (d *DB) writeSQLChunks(ctx context.Context, table string, names []string, byName map[string]colField, list []reflect.Value, size int, prepare bool) error {
	if size < 1 {
		size = 1
	}
	full := len(list) / size
	rem := len(list) % size
	var stmt *sql.Stmt
	fullSQL := ""
	if full >= 2 && prepare {
		fullSQL = d.dial.Rebind(insertValuesSQL(d.dial, table, names, size))
		var err error
		stmt, err = d.prepare(ctx, fullSQL)
		if err != nil {
			return err
		}
		// 出错时也要关掉。余数语句执行前会先关，避免同一条连接上叠着两条语句。
		defer func() {
			if stmt != nil {
				_ = stmt.Close()
			}
		}()
	}
	for i := 0; i < full; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		vals, err := bindRows(nil, list[i*size:(i+1)*size], names, byName)
		if err != nil {
			return err
		}
		if stmt != nil {
			if err := d.execStmt(ctx, stmt, fullSQL, vals); err != nil {
				return err
			}
			continue
		}
		if err := d.execInsert(ctx, table, names, vals); err != nil {
			return err
		}
	}
	if stmt != nil {
		err := stmt.Close()
		stmt = nil
		if err != nil {
			return err
		}
	}
	if rem == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	vals, err := bindRows(nil, list[full*size:], names, byName)
	if err != nil {
		return err
	}
	return d.execInsert(ctx, table, names, vals)
}

// prepare 只使用当前会话已经持有的执行器。
func (d *DB) prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	if d == nil || d.pool == nil {
		return nil, fmt.Errorf("dbx: nil session")
	}
	if d.done {
		return nil, fmt.Errorf("dbx: transaction already finished")
	}
	if d.tx != nil {
		return d.tx.PrepareContext(ctx, query)
	}
	return d.pool.PrepareContext(ctx, query)
}

func (d *DB) execStmt(ctx context.Context, stmt *sql.Stmt, query string, vals []any) error {
	start := d.mark()
	res, err := stmt.ExecContext(ctx, vals...)
	var n int64
	if err == nil && res != nil {
		n, _ = res.RowsAffected()
	}
	d.finish(ctx, "exec", query, vals, n, err, start)
	return err
}

// execInsert 执行一条已经绑定好的多行 INSERT。vals 的长度必须是列数的整数倍。
func (d *DB) execInsert(ctx context.Context, table string, names []string, vals []any) error {
	rows := 1
	if n := len(names); n > 0 {
		rows = len(vals) / n
	}
	_, err := d.execBound(ctx, insertValuesSQL(d.dial, table, names, rows), vals)
	return err
}

// insertValuesSQL 生成中间态 INSERT，占位符是 ?。调用方再按引擎 Rebind。
func insertValuesSQL(dial Dialect, table string, names []string, rows int) string {
	if rows < 1 {
		rows = 1
	}
	rowPH := "(" + strings.TrimSuffix(strings.Repeat("?,", len(names)), ",") + ")"
	var b strings.Builder
	b.Grow(len(table) + len(names)*8 + rows*(len(rowPH)+1) + 32)
	b.WriteString("INSERT INTO ")
	b.WriteString(dial.QuoteIdent(table))
	b.WriteString(" (")
	for i, name := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(dial.QuoteIdent(name))
	}
	b.WriteString(") VALUES ")
	for i := 0; i < rows; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(rowPH)
	}
	return b.String()
}

// bindRows 按列名顺序把结构体行收成绑定参数。dst 容量够时复用，避免每批重新分配。
func bindRows(dst []any, rows []reflect.Value, names []string, byName map[string]colField) ([]any, error) {
	if cap(dst) < len(rows)*len(names) {
		dst = make([]any, 0, len(rows)*len(names))
	}
	dst = dst[:0]
	for _, rv := range rows {
		for _, name := range names {
			c := byName[name]
			fv, ok := walkField(rv, c.index, false)
			if !ok {
				dst = append(dst, nil)
				continue
			}
			val, err := bindValue(fv)
			if err != nil {
				return nil, err
			}
			dst = append(dst, val)
		}
	}
	return dst, nil
}

// StmtExec 在当前会话已持有的连接上预编译 query，交给 fn 逐次执行，返回前关闭语句。
// 供 BulkDialect 使用。fn 里不要再 Begin 或向池申请连接。
// 整段只记一条 exec 日志，参数不附在事件上，避免大批量把参数留在调用方的切片里。
func (d *DB) StmtExec(ctx context.Context, query string, fn func(exec func(args ...any) error) error) error {
	if fn == nil {
		return fmt.Errorf("dbx: nil statement function")
	}
	ctx = d.before(ctx, "exec", query)
	stmt, err := d.prepare(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	start := d.mark()
	var n int64
	err = fn(func(args ...any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return err
		}
		n++
		return nil
	})
	d.finish(ctx, "exec", query, nil, n, err, start)
	return err
}
