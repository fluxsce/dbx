// Package mysql registers the MySQL and MariaDB dialect.
// Usage: import _ "github.com/fluxsce/dbx/driver/mysql"
package mysql

import (
	"database/sql"

	"github.com/fluxsce/dbx/db"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	db.Register(dialect{}, open)
}

// dialect 实现 db.Dialect：占位符保持 ?，标识符用反引号。
type dialect struct{}

// Name 返回规范化引擎名 mysql。
func (dialect) Name() string { return "mysql" }

// Rebind MySQL 使用 ?，中间态无需转换。
func (dialect) Rebind(query string) string { return query }

// QuoteIdent 用反引号包裹标识符。
func (dialect) QuoteIdent(name string) string { return "`" + name + "`" }

// LimitSQL 使用 LIMIT/OFFSET。
func (dialect) LimitSQL() string { return "LIMIT @limit OFFSET @offset" }

func open(dsn string) (*sql.DB, error) {
	return sql.Open("mysql", dsn)
}
