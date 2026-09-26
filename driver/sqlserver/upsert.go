package sqlserver

import "strings"

// UpsertSQL 生成 MERGE。语句以分号结束，这是 SQL Server 对 MERGE 的要求。
func (d dialect) UpsertSQL(table string, columns, keys, updates []string, n int) string {
	if n < 1 {
		n = 1
	}
	var b strings.Builder
	b.WriteString("MERGE ")
	b.WriteString(d.QuoteIdent(table))
	b.WriteString(" AS target USING (VALUES ")
	row := "(" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(row)
	}
	b.WriteString(") AS src (")
	b.WriteString(joinQuoted(d, columns))
	b.WriteString(") ON ")
	for i, key := range keys {
		if i > 0 {
			b.WriteString(" AND ")
		}
		q := d.QuoteIdent(key)
		b.WriteString("target.")
		b.WriteString(q)
		b.WriteString("=src.")
		b.WriteString(q)
	}
	b.WriteString(" WHEN MATCHED THEN UPDATE SET ")
	for i, col := range updates {
		if i > 0 {
			b.WriteByte(',')
		}
		q := d.QuoteIdent(col)
		b.WriteString(q)
		b.WriteString("=src.")
		b.WriteString(q)
	}
	b.WriteString(" WHEN NOT MATCHED THEN INSERT (")
	b.WriteString(joinQuoted(d, columns))
	b.WriteString(") VALUES (")
	for i, col := range columns {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("src.")
		b.WriteString(d.QuoteIdent(col))
	}
	b.WriteString(");")
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
