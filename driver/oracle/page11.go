package oracle

import "strings"

// dialect11 用 ROW_NUMBER 分页，因为 Oracle 11g 没有 OFFSET/FETCH。
// QueryPage 与 SelectPage 会调用 PageSQL。手写查询应使用 db.PageSQL，不要自行追加 LimitSQL。
type dialect11 struct{}

// Name 返回规范化引擎名 oracle11g。
func (dialect11) Name() string { return "oracle11g" }

// Rebind 与 12c 相同，将 ? 转为 :1、:2。
func (dialect11) Rebind(query string) string { return dialect{}.Rebind(query) }

// QuoteIdent 与 12c 相同，使用双引号。
func (dialect11) QuoteIdent(name string) string { return dialect{}.QuoteIdent(name) }

// LimitSQL 返回空串。11g 不能追加后缀，分页由 PageSQL 改写整句。
func (dialect11) LimitSQL() string { return "" }

// PageSQL 用 ROW_NUMBER 包住查询。没有 ORDER BY 时按 ROWID 排序。
func (dialect11) PageSQL(query string) string {
	inner, order := splitTrailingOrderBy(query)
	if order == "" {
		order = "ORDER BY ROWID"
	}
	return "SELECT * FROM (SELECT t.*, ROW_NUMBER() OVER (" + order + ") AS dbx_rn FROM (" + inner + ") t) WHERE dbx_rn > @offset AND dbx_rn <= @offset + @limit"
}

// splitTrailingOrderBy returns the query without a depth-0 ORDER BY, and that clause.
// ORDER BY inside parentheses or quotes stays in the inner query.
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
