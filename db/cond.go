package db

import (
	"fmt"
	"strings"
	"unicode"
)

// Cond 是 Update / Delete 的定位条件。必须显式传入，禁止空 SQL。
type Cond struct {
	// SQL 为不含 WHERE 关键字的条件片段，占位符只写 @name。
	SQL string
	// Args 为条件参数，键与 @name 对应且不含 @。
	Args Args
}

// Where 构造条件。sql 不要带 WHERE 关键字（带了也会去掉）。
func Where(sql string, args Args) Cond {
	return Cond{SQL: stripWhereKeyword(sql), Args: cloneArgs(args)}
}

// PK 按 row 的主键标签生成 Cond，与 Where 同一类型。
func (d *DB) PK(row any) (Cond, error) {
	rv, cols, err := structValue(row)
	if err != nil {
		return Cond{}, err
	}
	var (
		parts []string
		args  = Args{}
	)
	for _, c := range cols {
		if !c.pk {
			continue
		}
		fv, ok := walkField(rv, c.index, false)
		if !ok || isZero(fv) {
			return Cond{}, fmt.Errorf("dbx: pk %s is empty", c.column)
		}
		parts = append(parts, d.QuoteIdent(c.column)+" = @"+c.column)
		val, err := bindValue(fv)
		if err != nil {
			return Cond{}, err
		}
		args[c.column] = val
	}
	if len(parts) == 0 {
		return Cond{}, fmt.Errorf("dbx: mark pk with `db:\"col,pk\"`")
	}
	return Cond{SQL: strings.Join(parts, " AND "), Args: args}, nil
}

// normalized 去掉 WHERE 关键字、复制 Args；SQL 为空则报错，防止无条件改删全表。
func (c Cond) normalized() (Cond, error) {
	c.SQL = stripWhereKeyword(c.SQL)
	if c.SQL == "" {
		return Cond{}, fmt.Errorf("dbx: empty cond")
	}
	c.Args = cloneArgs(c.Args)
	return c, nil
}

// stripWhereKeyword 去掉开头的 WHERE（大小写不敏感）；wherefoo 这类标识符不剥。
func stripWhereKeyword(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 5 || !strings.EqualFold(s[:5], "where") {
		return s
	}
	rest := s[5:]
	if rest == "" {
		return ""
	}
	if !unicode.IsSpace(rune(rest[0])) {
		return s
	}
	return strings.TrimSpace(rest)
}
