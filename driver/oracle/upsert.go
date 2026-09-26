package oracle

import "strings"

// UpsertSQL 生成 MERGE。多行用 UNION ALL 接在 FROM dual 上。占位符保持 ?。
func (d dialect) UpsertSQL(table string, columns, keys, updates []string, n int) string {
	return mergeSQL(d.QuoteIdent, table, columns, keys, updates, n)
}

// UpsertSQL 与 12c 相同。11g 同样支持 MERGE。
func (d dialect11) UpsertSQL(table string, columns, keys, updates []string, n int) string {
	return mergeSQL(d.QuoteIdent, table, columns, keys, updates, n)
}

func mergeSQL(quote func(string) string, table string, columns, keys, updates []string, n int) string {
	if n < 1 {
		n = 1
	}
	var b strings.Builder
	b.WriteString("MERGE INTO ")
	b.WriteString(quote(table))
	b.WriteString(" target USING (")
	for row := 0; row < n; row++ {
		if row > 0 {
			b.WriteString(" UNION ALL ")
		}
		b.WriteString("SELECT ")
		for i := range columns {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('?')
			if row == 0 {
				b.WriteString(" AS ")
				b.WriteString(quote(columns[i]))
			}
		}
		b.WriteString(" FROM dual")
	}
	b.WriteString(") src ON (")
	for i, key := range keys {
		if i > 0 {
			b.WriteString(" AND ")
		}
		q := quote(key)
		b.WriteString("target.")
		b.WriteString(q)
		b.WriteString("=src.")
		b.WriteString(q)
	}
	b.WriteString(") WHEN MATCHED THEN UPDATE SET ")
	for i, col := range updates {
		if i > 0 {
			b.WriteByte(',')
		}
		q := quote(col)
		b.WriteString(q)
		b.WriteString("=src.")
		b.WriteString(q)
	}
	b.WriteString(" WHEN NOT MATCHED THEN INSERT (")
	for i, col := range columns {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(quote(col))
	}
	b.WriteString(") VALUES (")
	for i, col := range columns {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("src.")
		b.WriteString(quote(col))
	}
	b.WriteByte(')')
	return b.String()
}
