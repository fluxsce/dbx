// Package sqlserver registers the SQL Server dialect.
// Usage: import _ "github.com/fluxsce/dbx/driver/sqlserver"
//
// Business SQL stays @name. Rebind turns the internal ? into @p1, @p2.
// The sqlserver driver does not rewrite placeholders.
// Paging is OFFSET/FETCH and the SELECT must already contain ORDER BY.
package sqlserver

import (
	"database/sql"

	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/utils"
	_ "github.com/microsoft/go-mssqldb"
)

func init() {
	db.Register(dialect{}, open)
}

type dialect struct{}

// Name 返回规范化引擎名 sqlserver。
func (dialect) Name() string { return "sqlserver" }

// Rebind 把中间态 ? 换成 @p1、@p2。sqlserver 驱动不会做这一步。
func (dialect) Rebind(query string) string { return utils.RebindAtP(query) }

// QuoteIdent 用方括号包裹标识符，内部的 ] 写成 ]]。
func (dialect) QuoteIdent(name string) string { return utils.QuoteBracket(name) }

// LimitSQL 使用 OFFSET/FETCH。OrderBy 为空时，原 SQL 必须已有 ORDER BY。
func (dialect) LimitSQL() string { return "OFFSET @offset ROWS FETCH NEXT @limit ROWS ONLY" }

// InsertLimit：一条语句最多 2100 个参数，这里留出余量。多批同一事务，满批可预编译。
func (dialect) InsertLimit() db.InsertLimit {
	return db.InsertLimit{MaxRows: 1000, MaxParams: 2000, Atomic: true, Prepare: true}
}

func open(dsn string) (*sql.DB, error) {
	return sql.Open("sqlserver", dsn)
}
