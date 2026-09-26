// Package oracle registers Oracle 12c+ (FETCH) and Oracle 11g (ROW_NUMBER).
//
// The default build does not link godror, so publishing and CI do not need
// Oracle Instant Client. Connect with:
//
//	go build -tags oracle
//
// and a blank import of this package. Without the tag, Open returns an error
// that names the missing tag. Driver name oracle11g uses the same opener.
package oracle

import (
	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/utils"
)

func init() {
	db.Register(dialect{}, open)
	db.Register(dialect11{}, open)
}

type dialect struct{}

// Name 返回规范化引擎名 oracle。12c 及以上使用本方言。
func (dialect) Name() string { return "oracle" }

// Rebind 将中间态 ? 转为 :1、:2。
func (dialect) Rebind(query string) string { return utils.RebindColon(query) }

// QuoteIdent 用双引号包裹标识符。
func (dialect) QuoteIdent(name string) string { return utils.Quote(name, `"`) }

// LimitSQL 使用 OFFSET/FETCH。OrderBy 为空时，原 SQL 必须已有 ORDER BY。
func (dialect) LimitSQL() string { return "OFFSET @offset ROWS FETCH NEXT @limit ROWS ONLY" }

// InsertLimit：绑定变量上限 65535。单批 200 行，多批同一事务，满批可预编译。
func (dialect) InsertLimit() db.InsertLimit {
	return db.InsertLimit{MaxRows: 200, MaxParams: 65535, Atomic: true, Prepare: true}
}
