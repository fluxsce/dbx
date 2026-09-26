package db

import (
	"fmt"
	"strings"
)

// applyOrder 用 page.OrderBy 替换最外层 ORDER BY。空列表示沿用 query 里已有的排序。
func (d *DB) applyOrder(query string, page Page) (string, error) {
	col := strings.TrimSpace(page.OrderBy)
	if col == "" {
		return query, nil
	}
	if !validIdent(col) {
		return "", fmt.Errorf("dbx: invalid order column %q", page.OrderBy)
	}
	if d == nil || d.dial == nil {
		return "", fmt.Errorf("dbx: nil session")
	}
	dir := "ASC"
	if page.Desc {
		dir = "DESC"
	}
	inner, _ := splitTrailingOrderBy(query)
	return inner + " ORDER BY " + d.QuoteIdent(col) + " " + dir, nil
}

// splitTrailingOrderBy 拆出深度为 0 的最后一个 ORDER BY。括号和引号里的不拆。
func splitTrailingOrderBy(query string) (inner, order string) {
	idx := -1
	depth := 0
	inSingle, inDouble := false, false
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case inSingle:
			if c == '\'' {
				if i+1 < len(query) && query[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && matchOrderBy(query, i) {
				idx = i
			}
		}
	}
	if idx < 0 {
		return strings.TrimSpace(query), ""
	}
	return strings.TrimSpace(query[:idx]), strings.TrimSpace(query[idx:])
}

func matchOrderBy(q string, i int) bool {
	const kw = "order by"
	if i+len(kw) > len(q) || !strings.EqualFold(q[i:i+len(kw)], kw) {
		return false
	}
	if i > 0 && isIdentByte(q[i-1]) {
		return false
	}
	end := i + len(kw)
	if end < len(q) && isIdentByte(q[end]) {
		return false
	}
	return true
}

func isIdentByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
