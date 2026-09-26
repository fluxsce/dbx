# 更新日志

本文件记录了本项目的所有重要变更。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，并遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范。

## [Unreleased]

## [1.0.2] - 2026-09-26

### 变更
- **文档**：README 只保留安装、示例和引擎入口。约定、引擎对照、版本、从 sqlx 迁入和语句观测放在 `docs/`，英文为 `.en.md`，中文为 `.zh-CN.md`。
- **CI**：拉取请求和 `main` 上的测试拉起 MySQL 8.4、ClickHouse 24.8、PostgreSQL 16 和 SQL Server 2022，跑活库场景。SQLite 仍由同一次 `go test` 执行。Oracle 不进这条流水线。

## [1.0.1] - 2026-09-26

### 变更
- **`Upsert` 与错误分类**：`Upsert` 单独执行「没有则插入、已有则更新」。PostgreSQL 与 SQLite 用 `ON CONFLICT`，MySQL 用 `ON DUPLICATE KEY UPDATE`，SQL Server 与 Oracle 用 `MERGE`。ClickHouse 不提供该方法。`Classify` 把唯一冲突、死锁、锁等待和序列化失败收成 `ErrorKind`；`Retryable` 只对后三类为真。原始驱动错误不包装。
- **`IN (@name)` 展开切片**：参数是切片或数组时写成多个位置参数，再由方言改成 `?`、`$1` 或 `:1`。空切片报错。`[]byte` 仍是一个参数。切片写在 `IN` 之外会报错。
- **会话插件**：`Config.Plugins` 按可选接口挂到一条连接池上，并复制到它开出的事务。`EventPlugin` 在语句结束后收事件，`ContextPlugin` 在取连接前改写 context。实现了 `io.Closer` 的插件随连接池关闭。不设全局插件表。语句日志仍是内置的 `Config.Trace`，不放进插件列表。
- **批量 Insert 按引擎上限分批**：标准多行 INSERT 由 `InsertLimit` 声明单批行数、占位符、是否同一事务、能否预编译。满批语句在本次调用结束时关闭，事务中不再向连接池另取连接。另一种批量协议实现 `BulkDialect`，代码留在该驱动包；ClickHouse 的原生批量块即如此。未处理时仍走标准 INSERT。

## [1.0.0] - 2026-09-26

### 新增
- **一个会话同时是连接池和事务**：连接池上每条成功的语句自动提交。`Begin` 由调用方 `Commit` 或 `Rollback`。`Tx` 在回调返回 nil 时提交，返回错误或 panic 时回滚，panic 继续向外抛。同一个连接池可被多个 goroutine 使用；一次 `Begin` 得到的事务只能由一个 goroutine 使用。
- **业务 SQL 只写 @name**：参数键不加 `@`。占位符改写和标识符引号在 `utils`。会话、事务和结构体映射在 `db`。模块路径为 `github.com/fluxsce/dbx`，v1 不使用 `/v2` 后缀。
- **六种引擎各一个方言包**：SQLite、PostgreSQL、MySQL、ClickHouse、SQL Server。Oracle 12c 及更新版本走 `FETCH` 和 `:1`；`oracle11g` 用 `ROW_NUMBER` 改写整句。默认构建不链接 godror，连接时加 `-tags oracle`。ClickHouse 的更新和删除写成 `ALTER TABLE`。
- **结构体按 db 标签扫描**：支持结构体、`*struct`、`[]struct`、`[]*struct` 和匿名嵌入。`*T` 在非 NULL 时分配；NULL 写成零值或 nil 指针。支持 `sql.Scanner`、`bool`、整数、浮点、字符串、`[]byte`、`time.Time`，以及可选的 `record.Flag` 与 `record.Decimal`。无符号整数写入有符号字段时超出范围会报错。切片和 map 按元素转换。
- **Oracle 与 ClickHouse 专用类型留在方言包**：`NUMBER`、CLOB/BLOB、UUID、Decimal、大整数和 Geo 由各自方言收成文本或字节，再交给公共扫描。默认构建不导入 godror。
- **分页参数是 page、pageSize、OrderBy、Desc**：未传页大小时每页 20 条，库内不截断上限。分页子句写在各自方言包。URL 解析和列表 JSON 留在使用方。
- **日志和多数据源由调用方持有**：`Config.Trace` 在语句和事务结束后回调。慢查询和日志格式由调用方处理。多个数据源的连接池由调用方按名字持有。dbx 不设全局缓存，也不提供租户函数。
