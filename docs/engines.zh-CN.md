# 引擎

[English](engines.en.md)

业务 SQL 一律写 `@name`。下表是改写之后驱动收到的形态。补丁版可以改空白和子句顺序，并保持表中的形态。分批行数和占位符上限也可以在补丁版调整。

| 引擎 | 驱动占位符 | 标识符 | 分页 | Upsert | 更新 / 删除 |
|---|---|---|---|---|---|
| SQLite | `?` | `"` | `LIMIT` / `OFFSET` | `ON CONFLICT` | `UPDATE` / `DELETE` |
| PostgreSQL | `$1` | `"` | `LIMIT` / `OFFSET` | `ON CONFLICT` | `UPDATE` / `DELETE` |
| MySQL、MariaDB | `?` | `` ` `` | `LIMIT` / `OFFSET` | `ON DUPLICATE KEY UPDATE` | `UPDATE` / `DELETE` |
| SQL Server | `?`，再由驱动改成 `@p1` | `[` `]` | `OFFSET` / `FETCH` | `MERGE` | `UPDATE` / `DELETE` |
| Oracle 12c 及更新版本 | `:1` | `"` | `OFFSET` / `FETCH` | `MERGE` | `UPDATE` / `DELETE` |
| Oracle 11g | `:1` | `"` | 用 `ROW_NUMBER` 包住整句 | `MERGE` | `UPDATE` / `DELETE` |
| ClickHouse | `?` | `` ` `` | `LIMIT` / `OFFSET` | 不支持 | `ALTER TABLE … UPDATE` / `DELETE` |

`Driver: "oracle11g"` 走 Oracle 11g 那一行。`oracle` 和 `godror` 走 Oracle 12c。

`OFFSET` / `FETCH` 需要 `ORDER BY`。传入 `Page.OrderBy`，或在语句里自己写 `ORDER BY`。

MySQL 与 MariaDB 按实际撞上的唯一键更新。调用时传入的键名会对照结构体检查，不会写进语句。

ClickHouse 没有 `Upsert`。它的更新和删除是 mutation。事务不会撤销已经执行的语句。连接池上的多行 `Insert` 使用原生批量写入。

Oracle 需要 CGO、Oracle Instant Client 和 `-tags oracle`。默认的 `go test ./...` 不编译该驱动。ClickHouse 是纯 Go，只有导入之后才会链进程序。

新引擎实现 `db.Dialect`（`Name`、`Rebind`、`QuoteIdent`、`LimitSQL`），并用 `db.Register` 登记打开函数。变更语句、整句分页、分批上限、批量插入、upsert、错误分类和扫描转换走可选接口。会话代码留在 `db`。
