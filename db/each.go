package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// Each 流式扫描：每行覆盖 dest（非空 *struct）后调用 fn，不把整表装进切片。
// 适合大数据导出；query 可以没有 WHERE（整表）。内部关闭 Rows，调用方不必 Close。
// dest 在各行之间复用，fn 若异步使用必须先拷贝。fn 返回错误则停止。ctx 取消会中断。
func (d *DB) Each(ctx context.Context, dest any, query string, args Args, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("dbx: Each fn is required")
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("dbx: dest must be non-nil *struct")
	}
	sv := rv.Elem()
	if sv.Kind() != reflect.Struct {
		return fmt.Errorf("dbx: dest must be *struct")
	}
	sch, err := loadSchema(sv.Type())
	if err != nil {
		return err
	}
	rows, err := d.Query(ctx, query, args)
	if err != nil {
		return err
	}
	defer rows.Close() // Each 读完或中断都归还连接
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
		if err := ctx.Err(); err != nil {
			return err
		}
		sv.Set(reflect.Zero(sv.Type())) // 避免上一行残留到本行列缺失/NULL
		if err := scanRow(sv, sch, names, raw, ptrs, rows, d.dial); err != nil {
			return err
		}
		if err := fn(); err != nil {
			return err
		}
	}
	return rows.Err()
}

// EachTable 按结构体标签 SELECT 整表并流式回调，不带 WHERE。
// orderBy 建议填主键或业务序（如 `demoId`、`tenantId, demoId DESC`），空则不写 ORDER BY。
func (d *DB) EachTable(ctx context.Context, dest any, table, orderBy string, fn func() error) error {
	if err := validTableErr(table); err != nil {
		return err
	}
	cols, err := d.QuoteColumns(dest)
	if err != nil {
		return err
	}
	q := "SELECT " + cols + " FROM " + d.QuoteIdent(table)
	if strings.TrimSpace(orderBy) != "" {
		ob, err := d.quoteOrderBy(orderBy)
		if err != nil {
			return fmt.Errorf("dbx: EachTable: %w", err)
		}
		q += " ORDER BY " + ob
	}
	return d.Each(ctx, dest, q, nil, fn)
}

// quoteOrderBy 校验并引用 ORDER BY 片段：列名[, 列名 [ASC|DESC]]，禁止注入。
func (d *DB) quoteOrderBy(s string) (string, error) {
	var parts []string
	for _, raw := range strings.Split(s, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return "", fmt.Errorf("empty order by item")
		}
		fields := strings.Fields(raw)
		if len(fields) == 0 || len(fields) > 2 {
			return "", fmt.Errorf("invalid order by %q", raw)
		}
		if !validIdent(fields[0]) {
			return "", fmt.Errorf("invalid order column %q", fields[0])
		}
		item := d.QuoteIdent(fields[0])
		if len(fields) == 2 {
			dir := strings.ToUpper(fields[1])
			if dir != "ASC" && dir != "DESC" {
				return "", fmt.Errorf("invalid order direction %q", fields[1])
			}
			item += " " + dir
		}
		parts = append(parts, item)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("empty order by")
	}
	return strings.Join(parts, ", "), nil
}
