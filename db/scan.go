package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// Get 执行查询并将第一行扫入 dest（非空 *struct）。
// 无行返回 sql.ErrNoRows；查询列按名称匹配 `db` 标签，多出的列忽略。
func (d *DB) Get(ctx context.Context, dest any, query string, args Args) error {
	rows, err := d.Query(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close() // Get 内部关闭，调用方不必 Close
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := scanStruct(d.dial, rows, dest); err != nil {
		return err
	}
	return rows.Err()
}

// Select 将全部行扫入 dest（*[]struct 或 *[]*struct）。
// 每次覆盖 dest，不追加；内部关闭 Rows，调用方不必 Close。
func (d *DB) Select(ctx context.Context, dest any, query string, args Args) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("dbx: dest must be non-nil *[]T")
	}
	sv := rv.Elem()
	if sv.Kind() != reflect.Slice {
		return fmt.Errorf("dbx: dest must be *[]T")
	}
	rows, err := d.Query(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close() // 扫描中途失败也会归还连接
	elemType := sv.Type().Elem()
	ptrElem := elemType.Kind() == reflect.Pointer
	if ptrElem {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		return fmt.Errorf("dbx: slice element must be struct")
	}
	if sv.IsNil() {
		sv.Set(reflect.MakeSlice(sv.Type(), 0, 0))
	} else {
		sv.Set(sv.Slice(0, 0)) // 覆盖 dest，不追加上次结果
	}
	sch, err := loadSchema(elemType)
	if err != nil {
		return err
	}
	names, err := rows.Columns()
	if err != nil {
		return err
	}
	raw := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	for rows.Next() {
		item := reflect.New(elemType)
		if err := scanRow(item.Elem(), sch, names, raw, ptrs, rows, d.dial); err != nil {
			return err
		}
		if ptrElem {
			sv.Set(reflect.Append(sv, item))
		} else {
			sv.Set(reflect.Append(sv, item.Elem()))
		}
	}
	return rows.Err()
}

// SelectPage 先执行 countSQL 得到总数，再把当前页扫入 dest（*[]struct）。
// 分页由 PageSQL 决定：多数引擎追加 LimitSQL，Oracle 11g 改写整句。
func (d *DB) SelectPage(ctx context.Context, dest any, countSQL, listSQL string, page Page, args Args) (int64, error) {
	paged, err := d.PageSQL(listSQL, page)
	if err != nil {
		return 0, err
	}
	row, err := d.QueryRow(ctx, countSQL, args)
	if err != nil {
		return 0, err
	}
	var total int64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	err = d.Select(ctx, dest, paged, BindPage(args, page))
	return total, err
}

// Columns 返回结构体 `db` 标签列名（逗号分隔，不加引号）。
// 手写 SELECT 在 Postgres / Oracle 上应改用 QuoteColumns，以免未引用标识符被折成小写或大写。
func Columns(row any) (string, error) {
	sch, err := loadSchema(reflect.TypeOf(row))
	if err != nil {
		return "", err
	}
	names := make([]string, len(sch.cols))
	for i, c := range sch.cols {
		names[i] = c.column
	}
	return strings.Join(names, ", "), nil
}

// QuoteColumns 返回当前引擎引用后的列清单，供手写 SELECT 与 camelCase 列名跨库对齐。
func (d *DB) QuoteColumns(row any) (string, error) {
	sch, err := loadSchema(reflect.TypeOf(row))
	if err != nil {
		return "", err
	}
	names := make([]string, len(sch.cols))
	for i, c := range sch.cols {
		names[i] = d.QuoteIdent(c.column)
	}
	return strings.Join(names, ", "), nil
}

// scanStruct 把当前行扫进 dest（非空 *struct），供 Get 使用。
func scanStruct(dial Dialect, rows *sql.Rows, dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("dbx: scan dest must be non-nil *struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("dbx: scan dest must be *struct")
	}
	sch, err := loadSchema(rv.Type())
	if err != nil {
		return err
	}
	names, err := rows.Columns()
	if err != nil {
		return err
	}
	raw := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	return scanRow(rv, sch, names, raw, ptrs, rows, dial)
}

// scanRow 复用 raw/ptrs 缓冲扫一行，再按列名赋给结构体字段。
func scanRow(rv reflect.Value, sch *structSchema, names []string, raw []any, ptrs []any, rows *sql.Rows, dial Dialect) error {
	for i := range raw {
		raw[i] = nil // 避免上一行的非 NULL 值留在本行 NULL 列
	}
	if err := rows.Scan(ptrs...); err != nil {
		return err
	}
	for i, name := range names {
		cf, ok := lookupColumn(sch.byName, sch.byLower, name)
		if !ok {
			continue
		}
		fv, ok := walkField(rv, cf.index, true)
		if !ok {
			return fmt.Errorf("dbx: column %s: cannot set field", name)
		}
		src := raw[i]
		if c, ok := dial.(ScanDialect); ok {
			// 驱动专用类型在方言内转换；公共包不识别 godror / ClickHouse 类型
			out, handled, err := c.ConvertScan(src, fv.Type())
			if err != nil {
				return fmt.Errorf("dbx: column %s: %w", name, err)
			}
			if handled {
				src = out
			}
		}
		if err := assignField(fv, src); err != nil {
			return fmt.Errorf("dbx: column %s: %w", name, err)
		}
	}
	return nil
}
