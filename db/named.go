package db

import (
	"fmt"
	"unicode"
)

// Args 命名参数表。SQL 写 @tenantId，键写 tenantId（不要带 @）。
type Args map[string]any

// cloneArgs 复制参数表，避免 BindPage / Where 改到调用方传入的 map。
func cloneArgs(args Args) Args {
	out := Args{}
	for k, v := range args {
		out[k] = v
	}
	return out
}

// compile 将业务 SQL 的 @name 展开为位置参数，再交给 Dialect.Rebind。
// 业务侧禁止写 ? / $n；? 仅作内部中间态。@@ 转义为字面量 @。
// 字符串字面量、标识符引号与注释中的 @ / ? 不参与占位符。
func compile(d Dialect, query string, args Args) (string, []any, error) {
	q, vals, err := expandNamed(query, args)
	if err != nil {
		return "", nil, err
	}
	return d.Rebind(q), vals, nil
}

// expandNamed 把 @name 换成 ? 并按出现顺序取出参数，不 Rebind。
func expandNamed(query string, args Args) (string, []any, error) {
	if args == nil {
		args = Args{}
	}
	if err := rejectPositional(query); err != nil {
		return "", nil, err
	}
	var (
		out  []byte
		vals []any
		i    int
	)
	for i < len(query) {
		if n := literalSpan(query, i); n > 0 {
			out = append(out, query[i:i+n]...)
			i += n
			continue
		}
		if query[i] == '@' {
			if i+1 < len(query) && query[i+1] == '@' {
				out = append(out, '@') // @@ → 字面量 @
				i += 2
				continue
			}
			j := i + 1
			if j >= len(query) || !isIdentStart(rune(query[j])) {
				return "", nil, fmt.Errorf("dbx: invalid @ placeholder at %d", i)
			}
			j++
			for j < len(query) && isIdentPart(rune(query[j])) {
				j++
			}
			name := query[i+1 : j]
			v, ok := args[name]
			if !ok {
				return "", nil, fmt.Errorf("dbx: missing arg %q", name)
			}
			bound, err := bindArg(v)
			if err != nil {
				return "", nil, fmt.Errorf("dbx: arg %q: %w", name, err)
			}
			out = append(out, '?') // 中间态一律 ?，由 compile / execBound 再 Rebind
			vals = append(vals, bound)
			i = j
			continue
		}
		out = append(out, query[i])
		i++
	}
	return string(out), vals, nil
}

// rejectPositional 禁止业务 SQL 使用 ?，避免与 @name 混用、在 Postgres 上被误编成 $n。
func rejectPositional(query string) error {
	for i := 0; i < len(query); {
		if n := literalSpan(query, i); n > 0 {
			i += n
			continue
		}
		if query[i] == '?' {
			return fmt.Errorf("dbx: use @name placeholders, not ?")
		}
		i++
	}
	return nil
}

// literalSpan 返回从 i 起应原样拷贝的字面量/注释长度；0 表示此处不是字面量。
func literalSpan(q string, i int) int {
	if i >= len(q) {
		return 0
	}
	switch q[i] {
	case '\'', '"', '`':
		return quotedSpan(q, i, q[i])
	case '[':
		return bracketIdentSpan(q, i)
	case '-':
		if i+1 < len(q) && q[i+1] == '-' {
			return lineCommentSpan(q, i)
		}
	case '/':
		if i+1 < len(q) && q[i+1] == '*' {
			return blockCommentSpan(q, i)
		}
	}
	return 0
}

// quotedSpan 扫描成对引号（含 SQL 的 ” 转义），返回包含起止引号的长度。
func quotedSpan(q string, i int, quote byte) int {
	j := i + 1
	for j < len(q) {
		if q[j] != quote {
			j++
			continue
		}
		if j+1 < len(q) && q[j+1] == quote {
			j += 2
			continue
		}
		return j + 1 - i
	}
	return len(q) - i
}

// bracketIdentSpan 扫描 [ident] 形式的标识符（SQL Server 风格）。
func bracketIdentSpan(q string, i int) int {
	j := i + 1
	for j < len(q) {
		if q[j] == ']' {
			return j + 1 - i
		}
		j++
	}
	return len(q) - i
}

// lineCommentSpan 扫描 -- 行注释至换行（不含换行符之后）。
func lineCommentSpan(q string, i int) int {
	j := i + 2
	for j < len(q) && q[j] != '\n' {
		j++
	}
	return j - i
}

// blockCommentSpan 扫描 /* */ 块注释；未闭合则吃到串尾。
func blockCommentSpan(q string, i int) int {
	j := i + 2
	for j+1 < len(q) {
		if q[j] == '*' && q[j+1] == '/' {
			return j + 2 - i
		}
		j++
	}
	return len(q) - i
}

// isIdentStart 报告 r 是否可作为 @name 的首字符（字母或下划线）。
func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

// isIdentPart 报告 r 是否可作为 @name 的后续字符（字母、数字或下划线）。
func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
