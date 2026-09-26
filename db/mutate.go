package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// Insert 按结构体 `db` 标签生成 INSERT。
// rows 可以是 *struct、[]struct、[]*struct 或 *[]struct；切片一次写入多行。
// 多行先交给 BulkDialect；驱动返回未处理时，再按 InsertLimit 写成标准多行 INSERT。
func (d *DB) Insert(ctx context.Context, table string, rows any) error {
	if err := validTableErr(table); err != nil {
		return err
	}
	list, err := asRowList(rows)
	if err != nil {
		return err
	}
	cols, err := loadSchema(list[0].Type())
	if err != nil {
		return err
	}
	names, err := insertColumns(list[0], cols.cols)
	if err != nil {
		return fmt.Errorf("dbx: insert %s: %w", table, err)
	}
	byName := map[string]colField{}
	for _, c := range cols.cols {
		byName[c.column] = c
	}
	if len(list) > 1 {
		if bulk, ok := d.dial.(BulkDialect); ok {
			var buf []any
			done, err := bulk.BulkInsert(ctx, d, table, names, len(list), func(i int) ([]any, error) {
				var berr error
				buf, berr = bindRows(buf, list[i:i+1], names, byName)
				return buf, berr
			})
			if err != nil || done {
				return err
			}
		}
	}
	return d.insertSQL(ctx, table, names, byName, list)
}

// insertColumns 按第一行决定写入哪些列；omitempty 为零则跳过（后续行同列缺值写 NULL）。
func insertColumns(rv reflect.Value, cols []colField) ([]string, error) {
	var names []string
	for _, c := range cols {
		fv, ok := walkField(rv, c.index, false)
		if c.omitEmpty && (!ok || isZero(fv)) {
			continue
		}
		names = append(names, c.column)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no columns")
	}
	return names, nil
}

// asRowList 把 *struct / []struct / []*struct / *[]struct 收成结构体值列表。
func asRowList(row any) ([]reflect.Value, error) {
	if row == nil {
		return nil, fmt.Errorf("dbx: row must be non-nil pointer to struct or a slice")
	}
	rv := reflect.ValueOf(row)
	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, fmt.Errorf("dbx: row must be non-nil pointer to struct or a slice")
		}
		ev := rv.Elem()
		if ev.Kind() == reflect.Struct {
			return []reflect.Value{ev}, nil
		}
		if ev.Kind() == reflect.Slice {
			return sliceRows(ev)
		}
	case reflect.Slice:
		return sliceRows(rv)
	}
	return nil, fmt.Errorf("dbx: row must be *struct, []struct, or []*struct")
}

// sliceRows 展开切片元素；空切片或 nil 指针元素返回错误。
func sliceRows(sv reflect.Value) ([]reflect.Value, error) {
	if sv.Len() == 0 {
		return nil, fmt.Errorf("dbx: insert: empty slice")
	}
	out := make([]reflect.Value, 0, sv.Len())
	for i := 0; i < sv.Len(); i++ {
		ev := sv.Index(i)
		if ev.Kind() == reflect.Pointer {
			if ev.IsNil() {
				return nil, fmt.Errorf("dbx: insert: nil element")
			}
			ev = ev.Elem()
		}
		if ev.Kind() != reflect.Struct {
			return nil, fmt.Errorf("dbx: insert: slice element must be struct")
		}
		out = append(out, ev)
	}
	return out, nil
}

// Update 按 Cond 定位并写入 row。Cond 必须由 PK 或 Where 给出，禁止空条件。
// SET 与 WHERE 各自绑定，同名 @col 互不影响（改主键时 Cond 用旧值、row 里放新值）。
func (d *DB) Update(ctx context.Context, table string, row any, cond Cond) error {
	if err := validTableErr(table); err != nil {
		return err
	}
	cond, err := cond.normalized()
	if err != nil {
		return fmt.Errorf("dbx: update %s: %w", table, err)
	}
	rv, cols, err := structValue(row)
	if err != nil {
		return err
	}
	setArgs := Args{}
	var sets []string
	for _, c := range cols {
		if c.noUpdate {
			continue
		}
		fv, ok := walkField(rv, c.index, false)
		if c.omitEmpty && (!ok || isZero(fv)) {
			continue
		}
		sets = append(sets, d.QuoteIdent(c.column)+" = @"+c.column)
		if !ok {
			setArgs[c.column] = nil
			continue
		}
		val, err := bindValue(fv)
		if err != nil {
			return err
		}
		setArgs[c.column] = val
	}
	if len(sets) == 0 {
		return fmt.Errorf("dbx: update %s: no columns to set", table)
	}
	// SET 与 WHERE 分开 expand，同名 @col 各用各的值（改主键时旧值在 Cond、新值在 row）。
	setSQL, setVals, err := expandNamed(strings.Join(sets, ", "), setArgs)
	if err != nil {
		return err
	}
	whereSQL, whereVals, err := expandNamed(cond.SQL, cond.Args)
	if err != nil {
		return err
	}
	q := d.mutationUpdate(d.QuoteIdent(table), setSQL, whereSQL)
	_, err = d.execBound(ctx, q, append(setVals, whereVals...))
	return err
}

// mutationUpdate 生成更新语句。实现了 MutationDialect 的引擎改写整句，其余引擎使用标准 UPDATE。
func (d *DB) mutationUpdate(table, setSQL, whereSQL string) string {
	if m, ok := d.dial.(MutationDialect); ok {
		return m.UpdateSQL(table, setSQL, whereSQL) // ClickHouse 等在方言内改成 ALTER TABLE
	}
	return "UPDATE " + table + " SET " + setSQL + " WHERE " + whereSQL
}

// Delete 按 Cond 删除。Cond 必须由 PK 或 Where 给出，禁止空条件。
func (d *DB) Delete(ctx context.Context, table string, cond Cond) error {
	if err := validTableErr(table); err != nil {
		return err
	}
	cond, err := cond.normalized()
	if err != nil {
		return fmt.Errorf("dbx: delete %s: %w", table, err)
	}
	q := d.mutationDelete(d.QuoteIdent(table), cond.SQL)
	_, err = d.Exec(ctx, q, cond.Args)
	return err
}

// mutationDelete 生成删除语句。实现了 MutationDialect 的引擎改写整句，其余引擎使用标准 DELETE。
func (d *DB) mutationDelete(table, where string) string {
	if m, ok := d.dial.(MutationDialect); ok {
		return m.DeleteSQL(table, where) // 非标准删除由方言生成
	}
	return "DELETE FROM " + table + " WHERE " + where
}
