// Package postgres registers the PostgreSQL dialect (pgx database/sql driver).
// Usage: import _ "github.com/fluxsce/dbx/driver/postgres"
package postgres

import (
	"database/sql"

	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/utils"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() {
	db.Register(dialect{}, open)
}

// dialect 实现 db.Dialect：? 转为 $n，标识符用双引号。
type dialect struct{}

// Name 返回规范化引擎名 postgres。
func (dialect) Name() string { return "postgres" }

// Rebind 将中间态 ? 转为 $1、$2。
func (dialect) Rebind(query string) string { return utils.RebindDollar(query) }

// QuoteIdent 用双引号包裹标识符。
func (dialect) QuoteIdent(name string) string { return `"` + name + `"` }

// LimitSQL 使用 LIMIT/OFFSET。
func (dialect) LimitSQL() string { return "LIMIT @limit OFFSET @offset" }

// InsertLimit：一条语句最多 65535 个参数。单批 1000 行，多批同一事务，满批可预编译。
func (dialect) InsertLimit() db.InsertLimit {
	return db.InsertLimit{MaxRows: 1000, MaxParams: 65535, Atomic: true, Prepare: true}
}

func open(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}
