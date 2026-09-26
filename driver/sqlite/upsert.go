package sqlite

import "strings"

// UpsertSQL 生成 INSERT ... ON CONFLICT DO UPDATE。占位符保持 ?。
func (d dialect) UpsertSQL(table string, columns, keys, updates []string, n int) string {
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
	b.WriteString(" ON CONFLICT (")
	b.WriteString(joinQuoted(d, keys))
	b.WriteString(") DO UPDATE SET ")
	for i, col := range updates {
		if i > 0 {
			b.WriteByte(',')
		}
		q := d.QuoteIdent(col)
		b.WriteString(q)
		b.WriteString("=excluded.")
		b.WriteString(q)
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
