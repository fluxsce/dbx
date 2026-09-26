# dbx

[English](README.md)

`github.com/fluxsce/dbx` 是一个 Go 数据库会话：一个 `*DB`、命名参数 `@name`、每种引擎一个方言插件。连接池和事务是同一种类型。

远程仓库：`https://github.com/fluxsce/dbx.git`

```text
dbx/
  db/                 # *DB：连接池、SQL、事务、增删改查
  utils/              # 占位符改写与标识符引号，驱动共用
  record/             # 可选列类型：Flag、Decimal、墙钟时间
  driver/
    sqlite/
    postgres/
    mysql/
    clickhouse/
    sqlserver/
    oracle/           # 默认不链接 godror，-tags oracle 才连接；同时注册 oracle 与 oracle11g
```

业务 SQL 只写 `@name`，参数键不加 `@`。不要写 `?` 或 `$1`。建表语句走 `Exec`。

一次 `Open` 是一个连接池。多个数据源由调用方用 `map[string]*db.DB` 按名字持有，并在进程退出时逐个 `Close`。dbx 不维护全局连接缓存。

`Config.Trace` 在每条语句以及 begin、commit、rollback 之后调用。nil 表示不记录。慢查询阈值和日志格式留在调用方。

## 自动提交和手动事务

连接池上的 `*DB`，每条成功的语句自动提交。多个 goroutine 可以共用这一个池。

`Begin` 返回的事务由调用方 `Commit` 或 `Rollback`。这个值只能给一个 goroutine 用，提交或回滚之后不能再执行 SQL。已经在事务里再 `Begin` 会返回错误。

`Tx` 是同一套事务：回调返回 nil 就提交，返回错误或 panic 就回滚。已经在事务里再调用 `Tx`，会加入当前事务，由外层决定提交。

多个 goroutine 可以各自 `d.Begin()`。它们不能共用同一个事务值。

```go
// 自动提交
_, err = d.Exec(ctx, `INSERT INTO users (id, name) VALUES (@id, @name)`, db.Args{"id": "u1", "name": "ada"})

// 调用方提交
tx, err := d.Begin(ctx)
if _, err = tx.Exec(ctx, `UPDATE users SET name=@name WHERE id=@id`, db.Args{"id": "u1", "name": "ada"}); err != nil {
    _ = tx.Rollback()
    return err
}
return tx.Commit()
```

换库只改 `Driver` 和 `DSN`，并空白导入对应的 `driver` 包。分页用 `QueryPage` / `SelectPage`，分页子句写在各自方言包里。不要在业务 SQL 里写死 `LIMIT`。参数名是 `page` 和 `pageSize`；小于 1 的 `pageSize` 按 20 条。`OrderBy` 替换最外层排序，`Desc` 为降序。dbx 不截断上限，也不解析 URL。ClickHouse 的更新和删除会改成 `ALTER TABLE`。SQL Server 保持 `?`，由驱动改成 `@p1`。Oracle 12c 使用 `:1` 和 `FETCH`。`oracle11g` 用 `ROW_NUMBER` 改写整句。要真正连上 Oracle，编译时加 `-tags oracle`，并安装 Oracle Instant Client。

占位符改写和标识符引号在 `utils`。`db` 继续负责连接、事务和结构体映射。

## 发布时不会撞车的部分

模块路径 `github.com/fluxsce/dbx` 和其他叫 dbx 的项目不是同一个导入路径，`go get` 不会拉错仓库。

ClickHouse 驱动是纯 Go，只有空白导入 `driver/clickhouse` 的程序才会链进去。Oracle 的 godror 需要 CGO 和 Instant Client。这个依赖放在 `-tags oracle` 后面，默认的 `go test ./...` 和 GitHub 发布不会编译它，所以不会把只使用 SQLite 或 MySQL 的构建打失败。

推到 `main` 时，变更记录里的版本如果已经有同名标签，发布步骤会跳过，不会重复打 `v1.0.0`。

## 字段类型

扫描按 `db` 标签把列写入结构体。和 gateway `pkg/database` 的对照如下。日常标量、指针、切片和 map 在 `db`。Oracle 与 ClickHouse 的专用类型在各自方言包里收成这些值。

| 字段 | dbx | gateway `sqlutils` |
|---|---|---|
| 结构体、`*struct` | 支持。`Get` 要 `*struct`，`Select` 要 `*[]struct` 或 `*[]*struct` | 支持切片和结构体指针 |
| 匿名嵌入结构体，含嵌入指针 | 展开 `db` 标签；nil 嵌入指针在扫描时分配 | 按字段反射展开 |
| 具名嵌套结构体 | 不展开。该字段需要自己实现 `sql.Scanner`，或改成匿名嵌入 | 同样按列对字段，不把一行扫进嵌套对象 |
| `*string`、`*int`、`*time.Time`、`*bool` 等 | NULL 保持 nil；非 NULL 分配一层指针再写入 | 对字符串、整数、浮点、布尔、时间分别写了指针分支 |
| 值类型 `string`、整数、浮点 | NULL 写成零值；`[]byte` 先当文本，再解析数字 | 同样处理 `[]byte` 到数字（MySQL DECIMAL） |
| `bool`、`*bool` | 接受 `bool`、0/1 整数、`true`/`false`/`0`/`1` 文本 | 驱动直接给出 `bool` 时可以写入 |
| `sql.NullString` 等 `sql.Null*` | 字段实现 `sql.Scanner` 即可 | 扫描前会构造对应的 `sql.Null*` |
| `time.Time` | 接受 `time.Time` 和日期时间文本；写入时零值变成 NULL，非零写成墙钟 `2006-01-02 15:04:05` | 另外解析 SQLite 带偏移的文本 |
| `record.Flag` | 只认 `Y`/`N`，空 Flag 写入 `N` | 无此类型，业务自己用字符串 |
| `record.Decimal` | 用字符串进出，空值写入 NULL | 常扫成 `float64` 或 `[]byte` |
| Oracle 12c+ 分页与 `:1` 占位 | `driver/oracle`，连接要 `-tags oracle` | godror，默认编译进网关 |
| ClickHouse `ALTER` 更新删除 | `driver/clickhouse` | 单独嵌入并重写批量插入 |
| Oracle `NUMBER`、CLOB、BLOB | `driver/oracle` 收成文本或 `[]byte`，不把 godror 编进默认构建 | `godror.Number` 与 LOB 转换写在 `sqlutils` |
| ClickHouse UUID、Decimal、Array、Map、大整数、Geo | UUID / Decimal / 大整数 / Geo 在 `driver/clickhouse` 收成文本；Array 和 Map 按元素写入切片和 map | 写在 `sqlutils`，并把无符号整数一律当成 ClickHouse 类型 |

写入侧：nil 指针绑定为 SQL NULL；`driver.Valuer` 会先取值。`record.Flag`、`record.Decimal`、`time.Time` 的写出规则见上表。

## 发布

版本从 **1.0.0** 起。远程仓库是 `https://github.com/fluxsce/dbx.git`。推送到 `main` 并通过测试后，工作流读取 `CHANGELOG.md` 里的第一个版本标题。若仓库还没有对应的 `vX.Y.Z` 标签，就打上这个标签并创建 GitHub Release。`go get` 使用的就是这个标签。

下一版先改 `CHANGELOG.md` 顶部的版本号，再推到 `main`。停留在 v1 时，导入路径仍然是 `github.com/fluxsce/dbx`。

变更记录见 [CHANGELOG.md](CHANGELOG.md)。许可证为 Apache 2.0。
