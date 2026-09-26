// Package utils 提供各方言共用的 SQL 文本函数。
// 会话、事务和结构体映射在 github.com/fluxsce/dbx/db。
package utils

import "strings"

// RebindDollar 把每个 ? 换成 $1、$2，供 PostgreSQL 使用。
// 不解析引号内的文本。字符串字面量里不要写单独的 ?。
func RebindDollar(query string) string {
	return rebind(query, "$")
}

// RebindColon 把每个 ? 换成 :1、:2，供 Oracle（godror）使用。
// 不解析引号内的文本。字符串字面量里不要写单独的 ?。
func RebindColon(query string) string {
	return rebind(query, ":")
}

// RebindAtP 把每个 ? 换成 @p1、@p2，供 SQL Server 的 sqlserver 驱动使用。
// 该驱动不改写占位符，序号参数必须已经是 @pN。
func RebindAtP(query string) string {
	return rebind(query, "@p")
}

// Quote 用 mark 包裹标识符，例如反引号或双引号。
// 名字里的 mark 会写成两个。
func Quote(name, mark string) string {
	if mark == "" {
		return name
	}
	return mark + strings.ReplaceAll(name, mark, mark+mark) + mark
}

// QuoteBracket 用方括号包裹 SQL Server 标识符。
// 名字里的 ] 写成 ]]。
func QuoteBracket(name string) string {
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}

// rebind 把未加引号的 ? 换成 prefix 加序号，如 $1 或 :1。
func rebind(query, prefix string) string {
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteString(prefix)
			writeInt(&b, n)
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

func writeInt(b *strings.Builder, n int) {
	if n < 10 {
		b.WriteByte(byte('0' + n))
		return
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	b.Write(buf[i:])
}
