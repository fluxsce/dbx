// Package sqlserver registers the SQL Server dialect.
// Usage: import _ "github.com/fluxsce/dbx/driver/sqlserver"
//
// Business SQL stays @name. This driver keeps ? and the mssql driver
// turns each ? into @p1, @p2, so those names never meet the caller's @name.
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

// Rebind 保持 ?。go-mssqldb 自己把 ? 改成 @p1，避免和业务 SQL 的 @name 冲突。
func (dialect) Rebind(query string) string { return query }

// QuoteIdent 用方括号包裹标识符，内部的 ] 写成 ]]。
func (dialect) QuoteIdent(name string) string { return utils.QuoteBracket(name) }

// LimitSQL 使用 OFFSET/FETCH。OrderBy 为空时，原 SQL 必须已有 ORDER BY。
func (dialect) LimitSQL() string { return "OFFSET @offset ROWS FETCH NEXT @limit ROWS ONLY" }

func open(dsn string) (*sql.DB, error) {
	return sql.Open("sqlserver", dsn)
}
