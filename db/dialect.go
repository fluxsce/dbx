package db

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Dialect 一种数据库相对 SQL 标准的差异面。
// 新增引擎：实现本接口并 Register，业务 SQL 与分页代码不用改。
type Dialect interface {
	// Name 为规范化引擎名，如 sqlite、postgres、mysql。
	Name() string
	// Rebind 把中间形态的 ? 转成该引擎占位符（Postgres 为 $1 $2）。
	Rebind(query string) string
	// QuoteIdent 引用标识符；动态拼接列名时使用。
	QuoteIdent(name string) string
	// LimitSQL 返回追加在 SELECT 末尾的分页子句，占位符固定为 @limit / @offset。
	LimitSQL() string
}

// opener 按 DSN 打开标准库连接池。
type opener func(dsn string) (*sql.DB, error)

var (
	regMu    sync.RWMutex
	dialects = map[string]Dialect{} // 规范化名 → 方言
	openers  = map[string]opener{}  // 规范化名 → 打开连接池
	// aliases 把配置里的俗称收到注册名，避免业务写三套 driver 字符串。
	aliases = map[string]string{
		"sqlite":     "sqlite",
		"sqlite3":    "sqlite",
		"pg":         "postgres",
		"postgresql": "postgres",
		"postgres":   "postgres",
		"mysql":      "mysql",
		"mariadb":    "mysql",
		"oracle":     "oracle",
		"godror":     "oracle",
		"oracle11g":  "oracle11g",
		"sqlserver":  "sqlserver",
		"mssql":      "sqlserver",
		"clickhouse": "clickhouse",
	}
)

// Register 注册一种引擎。官方驱动在 driver/*/init 中调用；第三方同样走这一入口。
// d 或 open 为空会 panic，属于初始化错误，不应发生在请求路径。
func Register(d Dialect, open opener) {
	if d == nil || open == nil {
		panic("dbx: Register nil")
	}
	name := NormalizeDriver(d.Name())
	if name == "" {
		panic("dbx: dialect name empty")
	}
	regMu.Lock()
	defer regMu.Unlock()
	dialects[name] = d
	openers[name] = open
}

// NormalizeDriver 将配置项收成注册名：PG / postgresql → postgres。
func NormalizeDriver(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if canon, ok := aliases[s]; ok {
		return canon
	}
	return s
}

// lookup 按别名解析后取方言与 opener；未 import 驱动包则失败。
func lookup(driver string) (Dialect, opener, error) {
	name := NormalizeDriver(driver)
	regMu.RLock()
	defer regMu.RUnlock()
	d, ok := dialects[name]
	o, ok2 := openers[name]
	if !ok || !ok2 {
		return nil, nil, fmt.Errorf("dbx: driver %q not registered; blank-import github.com/fluxsce/dbx/driver/sqlite (or postgres/mysql/sqlserver/clickhouse/oracle) or Register a dialect", driver)
	}
	return d, o, nil
}

// MutationDialect 由更新和删除不是标准 SQL 的引擎实现。
// ClickHouse 写成 ALTER TABLE。其它引擎不实现本接口。
type MutationDialect interface {
	// UpdateSQL 返回该引擎的更新语句。table 已加引号，setClause 与 where 仍是 @name 展开前的片段。
	UpdateSQL(table, setClause, where string) string
	// DeleteSQL 返回该引擎的删除语句。table 已加引号。
	DeleteSQL(table, where string) string
}

// PageDialect 在分页不能追加在 SELECT 末尾时改写整句。
// Oracle 11g 用 ROW_NUMBER 包一层。其它引擎不实现本接口，改为追加 LimitSQL。
type PageDialect interface {
	// PageSQL 返回改写后的分页查询。占位符仍是 @limit 与 @offset。
	PageSQL(query string) string
}

// ScanDialect 在公共赋值之前，把驱动专用值收成公共扫描能识别的值。
// handled 为 false 时保持原值。dst 是结构体字段类型，含指针。
// Oracle 的 NUMBER、LOB 与 ClickHouse 的 UUID、Decimal 写在各自方言包。
type ScanDialect interface {
	// ConvertScan 转换一列驱动值。out 交给公共赋值；转换失败返回 err。
	ConvertScan(src any, dst reflect.Type) (out any, handled bool, err error)
}
