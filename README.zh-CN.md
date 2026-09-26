# dbx

[English](README.md) · [仓库](https://github.com/fluxsce/dbx) · [变更记录](CHANGELOG.md) · [许可证](LICENSE)

Go 数据库会话：一个 `*DB`、命名参数 `@name`、每种引擎一个方言插件。连接池和事务是同一种类型。

## 安装

```bash
go get github.com/fluxsce/dbx@v1.0.0
```

模块路径是 `github.com/fluxsce/dbx`。v1 没有 `/v2` 后缀。只空白导入实际要链接的驱动。

## 会话

连接池上的 `*DB`，每条成功的语句自动提交。多个 goroutine 可以共用这一个池。

`Begin` 返回的事务由调用方 `Commit` 或 `Rollback`。这个值只在一个 goroutine 里使用，提交或回滚之后停止执行 SQL。已经在事务里再 `Begin` 会返回错误。

`Tx` 在回调返回 nil 时提交，返回错误或 panic 时回滚。回滚之后 panic 继续向外抛。已经在事务里再调用 `Tx`，会加入当前事务，由外层决定提交。

每次 `Begin` 得到各自的事务。goroutine 之间各自持有自己的事务值。

```go
import (
    "context"
    "database/sql"
    "time"

    "github.com/fluxsce/dbx/db"
    _ "github.com/fluxsce/dbx/driver/sqlite"
)

d, err := db.Open(ctx, db.Config{
    Driver:          "sqlite",
    DSN:             "app.db",
    MaxOpenConns:    20,
    MaxIdleConns:    5,
    ConnMaxLifetime: 30 * time.Minute,
    ConnMaxIdleTime: 10 * time.Minute,
})

_, err = d.Exec(ctx, `
    INSERT INTO users (id, name) VALUES (@id, @name)`,
    db.Args{"id": "u1", "name": "ada"},
)

tx, err := d.Begin(ctx)
if _, err = tx.Exec(ctx, `UPDATE users SET name=@name WHERE id=@id`, db.Args{"id": "u1", "name": "ada lovelace"}); err != nil {
    _ = tx.Rollback()
    return err
}
if err = tx.Commit(); err != nil {
    return err
}

err = d.Tx(ctx, func(tx *db.DB) error {
    return tx.Insert(ctx, "users", &row)
})
```

业务 SQL 只写 `@name`，参数键不加 `@`。建表语句走 `Exec`。

`BeginOptions` 和 `TxOptions` 接受 `*sql.TxOptions`。传 nil 时用引擎默认隔离级别。

一次 `Open` 是一个连接池。多个数据源由调用方用 `map[string]*db.DB` 按名字持有，并在进程退出时逐个 `Close`。

`Config.Trace` 在每条语句以及 begin、commit、rollback 之后调用。nil 表示不记录。慢查询阈值和日志格式留在调用方。

`Get`、`Select`、`Each` 会关闭结果集。`Query` 和 `QueryPage` 把结果集交给调用方关闭。

## 引擎

更换 `Driver` 和 `DSN`，并空白导入对应驱动。

| 导入 | 名称 |
|---|---|
| `github.com/fluxsce/dbx/driver/sqlite` | `sqlite`、`sqlite3` |
| `github.com/fluxsce/dbx/driver/postgres` | `postgres`、`postgresql`、`pg` |
| `github.com/fluxsce/dbx/driver/mysql` | `mysql`、`mariadb` |
| `github.com/fluxsce/dbx/driver/clickhouse` | `clickhouse` |
| `github.com/fluxsce/dbx/driver/sqlserver` | `sqlserver`、`mssql` |
| `github.com/fluxsce/dbx/driver/oracle` | `oracle`、`godror`、`oracle11g`（编译加 `-tags oracle`） |

ClickHouse 是纯 Go，只有空白导入之后才会链进程序。Oracle 需要 CGO、Oracle Instant Client 和 `-tags oracle`。默认的 `go test ./...` 不编译 godror。

新引擎实现 `db.Dialect`（`Name`、`Rebind`、`QuoteIdent`、`LimitSQL`），并用 `db.Register` 登记打开函数。占位符改写和标识符引号在 `utils`。会话、事务和结构体映射在 `db`。

ClickHouse 的更新和删除写成 `ALTER TABLE`。SQL Server 保持 `?`，由驱动改成 `@p1`。Oracle 12c 使用 `:1` 和 `FETCH`。`Driver: "oracle11g"` 用 `ROW_NUMBER` 改写整句。

`Insert` 和 `Update` 按当前引擎引用标识符。手写 SQL 使用 `d.QuoteColumns`。

## 分页

使用 `QueryPage`、`SelectPage` 或 `PageSQL`。分页子句由当前方言生成。`Page` 的字段是 `page`、`pageSize`、`OrderBy`、`Desc`。小于 1 的 `pageSize` 按 20 条。dbx 不截断上限，也不解析 URL。

`OrderBy` 替换最外层 `ORDER BY`。`Desc` 为降序。`OrderBy` 为空时，`FETCH` 语句本身需要已有 `ORDER BY`。

## 字段

`github.com/fluxsce/dbx/record` 可选。

| 列 | Go | 行为 |
|---|---|---|
| Y/N 标志 | `record.Flag` | 空 Flag 写入 `N` |
| DECIMAL / 金额 | `record.Decimal` | 空 Decimal 写入 NULL |
| 日期时间文本 | `time.Time` | 墙钟 `2006-01-02 15:04:05`；零值写入 NULL |

扫描按 `db` 标签匹配列。支持结构体、`*struct`、`[]struct`、`[]*struct` 和匿名嵌入。非 NULL 的 `*T` 会分配；NULL 写成零值或 nil 指针。支持 `sql.Scanner`（含 `sql.Null*`）、`bool`、整数、浮点、字符串和 `[]byte`。无符号整数写不进有符号字段时返回错误。切片和 map 在元素类型不同时逐个转换。

Oracle 的 `NUMBER` 和 CLOB/BLOB 在 `driver/oracle` 读取。ClickHouse 的 UUID、Decimal、大整数和 Geo 在 `driver/clickhouse` 读取。数组和 map 走公共的切片与 map 赋值。

结构体标签：`db:"name"`；联合主键 `,pk`；跳过 `db:"-"`；零值省略 `,omitempty`；不参与 UPDATE `,noupdate`。

`Update` 和 `Delete` 需要 `Cond`（`d.PK` 或 `db.Where`）。空条件返回错误。

nil 指针绑定为 SQL NULL。`driver.Valuer` 会先取值。

## 目录

```text
dbx/
  db/                 # *DB：连接池、SQL、事务、增删改查
  utils/              # 占位符改写与标识符引号
  record/             # 可选列类型：Flag、Decimal、墙钟时间
  driver/             # sqlite、postgres、mysql、clickhouse、sqlserver、oracle
```

## 项目

源码：<https://github.com/fluxsce/dbx>

版本从 1.0.0 起。推送到 `main` 并通过测试后，工作流读取 [CHANGELOG.md](CHANGELOG.md) 里第一个 `## [x.y.z] - 日期` 标题。该标签尚不存在时推上 `vX.Y.Z`，并创建 GitHub Release。`go get` 从这一个公开仓库拉取该标签，模块代理在第一次请求时缓存。

Apache License 2.0。见 [LICENSE](LICENSE)。
