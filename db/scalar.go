package db

import (
	"context"
)

// Count 执行返回单个计数值的查询（通常为 SELECT count(*) …），取第一列写成 int64。
// 无行时返回 sql.ErrNoRows（多数引擎的 count(*) 总会有一行）。
func (d *DB) Count(ctx context.Context, query string, args Args) (int64, error) {
	row, err := d.QueryRow(ctx, query, args)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// Exists 在查询至少返回一行时为 true。调用方应自行 LIMIT 1（或等价）以免扫全表。
// 无行返回 false, nil；语句错误原样返回。
func (d *DB) Exists(ctx context.Context, query string, args Args) (bool, error) {
	rows, err := d.Query(ctx, query, args)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	return true, rows.Err()
}
