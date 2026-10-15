package db

import "strings"

// CountSQL 由列表 SQL 生成瘦计数语句（对齐 gateway BuildCountQuery 思路，并按括号深度解析）。
//
// 正常分页只传 listSQL：SelectPage / QueryPage 内部调用本函数，不必再传 countSQL。
//
// 处理顺序：
//  1. 去掉最外层 ORDER BY / LIMIT / OFFSET / FETCH（COUNT 不需要排序与分页）
//  2. 无最外层 DISTINCT / GROUP BY / HAVING / UNION 等时：
//     SELECT … FROM … → SELECT COUNT(*) AS cnt FROM …
//     （丢掉宽投影，保留 JOIN / WHERE；别名 cnt 便于 SQL Server 等扫描）
//  3. 否则尽量收成 SELECT 1 再包一层；最外层 DISTINCT 保留原选择列表再包一层
//
// 仅为展示且不影响行数的 JOIN 无法可靠自动去掉；若实测 count 过重，再用 QueryPageCount。
func CountSQL(listSQL string) string {
	inner := stripCountNoise(strings.TrimSpace(listSQL))
	if inner == "" {
		return "SELECT COUNT(*) AS cnt FROM (SELECT 1 WHERE 0=1) AS _dbx_cnt"
	}
	if canRewriteCountStar(inner) {
		if fromRest, ok := splitSelectFrom(inner); ok {
			return "SELECT COUNT(*) AS cnt " + fromRest
		}
	}
	body := inner
	if !selectHasDistinct(inner) {
		if lean, ok := replaceSelectList(inner, "1"); ok {
			body = lean
		}
	}
	return "SELECT COUNT(*) AS cnt FROM (" + body + ") AS _dbx_cnt"
}

// stripCountNoise 去掉 COUNT 不需要的最外层排序与分页子句。
func stripCountNoise(query string) string {
	q, _ := splitTrailingOrderBy(query)
	for {
		next := q
		next = stripTrailingKeywordClause(next, "limit")
		next = stripTrailingKeywordClause(next, "offset")
		next = stripTrailingKeywordClause(next, "fetch")
		if next == q {
			return strings.TrimSpace(q)
		}
		q = next
	}
}

// stripTrailingKeywordClause 去掉深度为 0 的最后一个 kw 及其后内容（如 LIMIT / OFFSET / FETCH）。
func stripTrailingKeywordClause(query, kw string) string {
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
			if depth == 0 && matchKeywordAt(query, i, kw) {
				idx = i
			}
		}
	}
	if idx < 0 {
		return query
	}
	return strings.TrimSpace(query[:idx])
}

func canRewriteCountStar(query string) bool {
	return !selectHasDistinct(query) &&
		!hasDepth0Phrase(query, "group by") &&
		!hasDepth0Phrase(query, "having") &&
		!hasDepth0SetOp(query)
}

// splitSelectFrom 拆出最外层 SELECT 后的 FROM … 段（含 FROM 关键字）。
func splitSelectFrom(query string) (fromRest string, ok bool) {
	q := strings.TrimSpace(query)
	if !hasPrefixKeyword(q, "select") {
		return "", false
	}
	i := len("select")
	for i < len(q) && isSpace(q[i]) {
		i++
	}
	fromIdx := indexDepth0Keyword(q, i, "from")
	if fromIdx < 0 {
		return "", false
	}
	return strings.TrimSpace(q[fromIdx:]), true
}

// replaceSelectList 把最外层 SELECT 与 FROM 之间的选择列表换成 list（如 "1"）。
func replaceSelectList(query, list string) (string, bool) {
	q := strings.TrimSpace(query)
	if !hasPrefixKeyword(q, "select") {
		return "", false
	}
	i := len("select")
	for i < len(q) && isSpace(q[i]) {
		i++
	}
	prefix := q[:i]
	if hasPrefixKeyword(q[i:], "distinct") {
		i += len("distinct")
		for i < len(q) && isSpace(q[i]) {
			i++
		}
		prefix = q[:i]
	} else if hasPrefixKeyword(q[i:], "all") {
		i += len("all")
		for i < len(q) && isSpace(q[i]) {
			i++
		}
		prefix = q[:i]
	}
	fromIdx := indexDepth0Keyword(q, i, "from")
	if fromIdx < 0 {
		return "", false
	}
	return prefix + list + " " + strings.TrimSpace(q[fromIdx:]), true
}

func selectHasDistinct(query string) bool {
	q := strings.TrimSpace(query)
	if !hasPrefixKeyword(q, "select") {
		return false
	}
	i := len("select")
	for i < len(q) && isSpace(q[i]) {
		i++
	}
	return hasPrefixKeyword(q[i:], "distinct")
}

func hasDepth0SetOp(query string) bool {
	return hasDepth0Phrase(query, "union") ||
		hasDepth0Phrase(query, "intersect") ||
		hasDepth0Phrase(query, "except")
}

func hasDepth0Phrase(query, phrase string) bool {
	return indexDepth0Keyword(query, 0, phrase) >= 0
}

func indexDepth0Keyword(query string, from int, kw string) int {
	depth := 0
	inSingle, inDouble := false, false
	for i := from; i < len(query); i++ {
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
			if depth == 0 && matchKeywordAt(query, i, kw) {
				return i
			}
		}
	}
	return -1
}

func hasPrefixKeyword(s, kw string) bool {
	return matchKeywordAt(s, 0, kw)
}

func matchKeywordAt(q string, i int, kw string) bool {
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

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
