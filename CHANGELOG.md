# Changelog

## 1.0.0

第一个稳定版本。模块路径为 `github.com/fluxsce/dbx`，v1 不使用 `/v2` 后缀。

- 会话：连接池上的语句自动提交；`Begin` / `Commit` / `Rollback` 由调用方提交；`Tx` 在回调返回 nil 时提交。
- 并发：同一个连接池可被多个 goroutine 使用；一次 `Begin` 得到的事务只能由一个 goroutine 使用。
- SQL 只写 `@name`。官方驱动：SQLite、PostgreSQL、MySQL、ClickHouse、SQL Server。Oracle 12c 及更新版本走 `FETCH` 和 `:1`；`oracle11g` 用 `ROW_NUMBER` 改写整句。默认构建不链接 godror，连接时加 `-tags oracle`。
- 占位符改写和标识符引号在 `utils`。会话、事务和结构体映射仍在 `db`。ClickHouse 的更新和删除写成 `ALTER TABLE`。
- 字段：结构体、`*struct`、`[]struct`、`[]*struct`、匿名嵌入结构体；`*T` 在非 NULL 时分配；NULL 写成零值或 nil 指针；`sql.Scanner`（含 `sql.Null*`）；`bool`；整数、浮点、字符串、`[]byte`；`time.Time`；可选的 `record.Flag` 与 `record.Decimal`。无符号整数写入有符号字段时超出范围会报错。切片和 map 按元素转换。
- Oracle 的 `NUMBER` 和 CLOB/BLOB、ClickHouse 的 UUID、Decimal、大整数和 Geo 由各自方言收成文本或字节，再交给公共扫描。默认构建不导入 godror。
- 平台库的 tenantId 非空检查留在使用方。dbx 不提供租户函数。
- 分页参数是 `page`、`pageSize`、`OrderBy`、`Desc`。未传页大小时每页 20 条，库内不截断上限。分页子句写在各自方言包。URL 解析和列表 JSON 留在使用方。
- `Config.Trace` 在语句和事务结束后回调。慢查询和日志格式由调用方处理。多个数据源的连接池由调用方按名字持有，dbx 不设全局缓存。
