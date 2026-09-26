package db

import (
	"context"
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

// InsertLimit 是标准多行 INSERT 的单批上限。
// 新引擎只返回自己的上限，不必改 db。未实现 InsertDialect 时按 200 行、1000 个占位符、跨批同一事务，并允许预编译。
type InsertLimit struct {
	// MaxRows 是单批最多行数。小于 1 时用默认。
	MaxRows int
	// MaxParams 是单批最多占位符。小于 1 时用默认。列数很多时行数会再缩小。
	MaxParams int
	// Atomic 为 true 时，跨批写入放进同一事务，中途失败整批回滚。
	Atomic bool
	// Prepare 为 true 时，相同的满批语句在本次调用内预编译一次，返回前关闭。
	// 驱动把 Prepare 用作自己的批量协议时必须为 false，否则会占住提交回调。
	Prepare bool
}

// InsertDialect 声明标准多行 INSERT 怎么分批。批量协议不同的引擎再实现 BulkDialect。
type InsertDialect interface {
	InsertLimit() InsertLimit
}

// BulkDialect 由标准多行 VALUES 不合适的引擎实现，逻辑留在该驱动包。
// db 只在多行 Insert 时做类型断言。done 为 false 且 err 为 nil 时，改走标准多行 INSERT。
type BulkDialect interface {
	// BulkInsert 写入 n 行。row 返回与 columns 等长、已经绑定好的参数，只在被调用时取一行。
	// 实现要自己释放语句，并且不要在已持有的连接之外再向池申请连接。
	BulkInsert(ctx context.Context, sess *DB, table string, columns []string, n int, row func(i int) ([]any, error)) (done bool, err error)
}

// UpsertDialect 由「没有则插入、已有则更新」写成一条语句的引擎实现。
// ClickHouse 不实现：重复键不会在写入当时更新那一行。
// columns、keys、updates 都是未加引号的列名。n 是行数。占位符按行优先展开为 ?，由会话再 Rebind。
type UpsertDialect interface {
	// UpsertSQL 返回该引擎的 UPSERT 或 MERGE。n 小于 1 时按 1 行。
	UpsertSQL(table string, columns, keys, updates []string, n int) string
}

// ErrorDialect 把驱动错误收成跨库分类。未实现时 Classify 返回 ErrorOther。
// 原始错误不包装，调用方仍可用 errors.Is / errors.As 读取驱动类型。
type ErrorDialect interface {
	// Classify 识别唯一冲突、死锁、锁等待和序列化失败。其余返回 ErrorOther。
	Classify(err error) ErrorKind
}

// ScanDialect 在公共赋值之前，把驱动专用值收成公共扫描能识别的值。
// handled 为 false 时保持原值。dst 是结构体字段类型，含指针。
// Oracle 的 NUMBER、LOB 与 ClickHouse 的 UUID、Decimal 写在各自方言包。
type ScanDialect interface {
	// ConvertScan 转换一列驱动值。out 交给公共赋值；转换失败返回 err。
	ConvertScan(src any, dst reflect.Type) (out any, handled bool, err error)
}
