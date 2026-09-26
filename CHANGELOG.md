# 更新日志

本文件记录了本项目的所有重要变更。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，并遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范。

## [Unreleased]

## [1.0.0] - 2026-09-26

### 新增
- **一个会话同时是连接池和事务**：连接池上每条成功的语句自动提交。`Begin` 由调用方 `Commit` 或 `Rollback`。`Tx` 在回调返回 nil 时提交，返回错误或 panic 时回滚，panic 继续向外抛。同一个连接池可被多个 goroutine 使用；一次 `Begin` 得到的事务只能由一个 goroutine 使用。
- **业务 SQL 只写 @name**：参数键不加 `@`。占位符改写和标识符引号在 `utils`。会话、事务和结构体映射在 `db`。模块路径为 `github.com/fluxsce/dbx`，v1 不使用 `/v2` 后缀。
- **六种引擎各一个方言包**：SQLite、PostgreSQL、MySQL、ClickHouse、SQL Server。Oracle 12c 及更新版本走 `FETCH` 和 `:1`；`oracle11g` 用 `ROW_NUMBER` 改写整句。默认构建不链接 godror，连接时加 `-tags oracle`。ClickHouse 的更新和删除写成 `ALTER TABLE`。
- **结构体按 db 标签扫描**：支持结构体、`*struct`、`[]struct`、`[]*struct` 和匿名嵌入。`*T` 在非 NULL 时分配；NULL 写成零值或 nil 指针。支持 `sql.Scanner`、`bool`、整数、浮点、字符串、`[]byte`、`time.Time`，以及可选的 `record.Flag` 与 `record.Decimal`。无符号整数写入有符号字段时超出范围会报错。切片和 map 按元素转换。
- **Oracle 与 ClickHouse 专用类型留在方言包**：`NUMBER`、CLOB/BLOB、UUID、Decimal、大整数和 Geo 由各自方言收成文本或字节，再交给公共扫描。默认构建不导入 godror。
- **分页参数是 page、pageSize、OrderBy、Desc**：未传页大小时每页 20 条，库内不截断上限。分页子句写在各自方言包。URL 解析和列表 JSON 留在使用方。
- **日志和多数据源由调用方持有**：`Config.Trace` 在语句和事务结束后回调。慢查询和日志格式由调用方处理。多个数据源的连接池由调用方按名字持有。dbx 不设全局缓存，也不提供租户函数。
