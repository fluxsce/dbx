package mysql

import "strings"

// UpsertSQL 生成 INSERT ... ON DUPLICATE KEY UPDATE。
// MySQL 与 MariaDB 不能在语句里指定冲突键，VALUES(列) 取本次插入的值，两种库都能执行。
func (d dialect) UpsertSQL(table string, columns, _ []string, updates []string, n int) string {
	if n < 1 {
		n = 1
	}
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(d.QuoteIdent(table))
	b.WriteString(" (")
	b.WriteString(joinQuoted(d, columns))
	b.WriteString(") VALUES ")
	row := "(" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(row)
	}
	b.WriteString(" ON DUPLICATE KEY UPDATE ")
	for i, col := range updates {
		if i > 0 {
			b.WriteByte(',')
		}
		q := d.QuoteIdent(col)
		b.WriteString(q)
		b.WriteString("=VALUES(")
		b.WriteString(q)
		b.WriteByte(')')
	}
	return b.String()
}

func joinQuoted(d dialect, names []string) string {
	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(d.QuoteIdent(name))
	}
	return b.String()
}
