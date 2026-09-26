// Package clickhouse registers the ClickHouse dialect.
// Usage: import _ "github.com/fluxsce/dbx/driver/clickhouse"
//
// UPDATE and DELETE are rewritten to ALTER TABLE. Multi-row inserts on the
// pool use the native batch in this package. ClickHouse transactions do not
// undo a statement that already ran.
package clickhouse

import (
	"database/sql"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/utils"
)

func init() {
	db.Register(dialect{}, open)
}

type dialect struct{}

// Name 返回规范化引擎名 clickhouse。
func (dialect) Name() string { return "clickhouse" }

// Rebind ClickHouse 使用 ?，中间态无需转换。
func (dialect) Rebind(query string) string { return query }

// QuoteIdent 用反引号包裹标识符。
func (dialect) QuoteIdent(name string) string { return utils.Quote(name, "`") }

// LimitSQL 使用 LIMIT/OFFSET。
func (dialect) LimitSQL() string { return "LIMIT @limit OFFSET @offset" }

// InsertLimit 是事务内退回标准 INSERT 时的上限。
// Prepare 必须为 false：本驱动的 Prepare 会占用提交回调。
// 这类事务不会撤销已执行的 INSERT，所以跨批不再包一层事务。
func (dialect) InsertLimit() db.InsertLimit {
	return db.InsertLimit{MaxRows: 1000, MaxParams: 100000, Atomic: false, Prepare: false}
}

// UpdateSQL 把更新写成 ALTER TABLE ... UPDATE。
func (dialect) UpdateSQL(table, setClause, where string) string {
	return "ALTER TABLE " + table + " UPDATE " + setClause + " WHERE " + where
}

// DeleteSQL 把删除写成 ALTER TABLE ... DELETE。
func (dialect) DeleteSQL(table, where string) string {
	return "ALTER TABLE " + table + " DELETE WHERE " + where
}

func open(dsn string) (*sql.DB, error) {
	return sql.Open("clickhouse", dsn)
}
