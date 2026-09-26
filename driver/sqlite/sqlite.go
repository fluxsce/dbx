// Package sqlite registers the SQLite dialect (modernc.org/sqlite, no cgo).
// Usage: import _ "github.com/fluxsce/dbx/driver/sqlite"
package sqlite

import (
	"database/sql"

	"github.com/fluxsce/dbx/db"
	_ "modernc.org/sqlite"
)

func init() {
	db.Register(dialect{}, open)
}

// dialect 实现 db.Dialect：占位符保持 ?，标识符用双引号。
type dialect struct{}

// Name 返回规范化引擎名 sqlite。
func (dialect) Name() string { return "sqlite" }

// Rebind SQLite 使用 ?，中间态无需转换。
func (dialect) Rebind(query string) string { return query }

// QuoteIdent 用双引号包裹标识符。
func (dialect) QuoteIdent(name string) string { return `"` + name + `"` }

// LimitSQL 使用 LIMIT/OFFSET。
func (dialect) LimitSQL() string { return "LIMIT @limit OFFSET @offset" }

// InsertLimit：变量上限 32766，单批再收到 500 行，避免一条 SQL 过大。
// 多批放进同一事务，中途失败整批回滚。满批语句可以预编译。
func (dialect) InsertLimit() db.InsertLimit {
	return db.InsertLimit{MaxRows: 500, MaxParams: 32766, Atomic: true, Prepare: true}
}

func open(dsn string) (*sql.DB, error) {
	return sql.Open("sqlite", dsn)
}
