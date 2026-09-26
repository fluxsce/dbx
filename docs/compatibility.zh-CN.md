# 兼容性

[English](compatibility.en.md)

本页是 1.x 的约定。按此编写的程序在所有 1.x 发布中保持可编译，下面的会话行为保持不变。打破本约定即升主版本。见 [版本](versioning.zh-CN.md)。

各引擎的 SQL 形态见 [引擎](engines.zh-CN.md)。补丁版可以改生成 SQL 的具体文本，并保持该页列出的形态。

## 稳定行为

- 业务 SQL 使用 `@name`。`Args` 的键不带 `@`。`@@` 是字面量 `@`。业务 SQL 里的 `?` 占位符返回错误。引号、注释和方括号标识符保持原样。
- `IN (@name)` 把切片或数组展开成每个元素一个占位符。空切片返回错误。`[]byte` 是一个值。切片写在 `IN` 之外返回错误。
- 连接池上的 `*DB` 在每条成功语句后提交，可以被多个 goroutine 使用。事务 `*DB` 在 `Commit` 或 `Rollback` 之前只由一个 goroutine 使用。
- 已在事务中再 `Begin` 返回错误。已在事务中再 `Tx`，回调跑在当前事务上，由外层提交或回滚。内层传入的事务选项被忽略。
- `Tx` 在回调返回 nil 时提交。回调返回错误或 panic 时回滚，然后 panic 继续向外抛。
- `Update` 和 `Delete` 的条件必须来自 `PK` 或 `Where`。空条件返回错误。`SET` 与 `WHERE` 分开绑定，因此同一个名字可以在行里是新值、在条件里是旧值。
- `Get`、`Select`、`Each` 关闭结果集。`Query` 和 `QueryPage` 把结果集交给调用方关闭。`Select` 覆盖目标切片。
- 扫描按列名匹配 `db` 标签。多出来的列忽略。没有 `db` 标签的导出字段不是列。`db:"-"` 跳过该字段。
- `Classify` 返回 `ErrorKind`，不包装驱动错误。`errors.Is` 和 `errors.As` 仍能看到原始错误。死锁、锁等待和序列化失败的 `Retryable` 为真。唯一冲突不可按原样重试。
- MySQL 与 MariaDB 的 `Upsert` 按实际撞上的唯一键更新那一行，语句里不能指定键名。其它支持 `Upsert` 的引擎，以调用时传入的键列作为冲突目标。
- `OrderBy` 和 `EachTable` 的排序只接受标识符，并由当前引擎加引号。

## 结构体标签

| 标签 | 作用 |
|---|---|
| `db:"name"` | 列名。没有该标签的导出字段不是列。 |
| `,pk` | `PK` 使用的键列。联合键是多个带 `,pk` 的字段。 |
| `,omitempty` | 零值不进入 `INSERT` 和 `UPDATE`。 |
| `,noupdate` | 列会插入，不进入 `UPDATE`，也不进入 `Upsert` 的更新列表。 |
| `db:"-"` | 该字段不是列。 |

匿名嵌入结构体会展开。未导出字段跳过。

`Insert`、`Update`、`Upsert` 接受 `*struct`、`[]struct`、`[]*struct` 和 `*[]struct`。`Get` 和 `Each` 扫进 `*struct`。`Select` 和 `SelectPage` 扫进 `*[]struct` 或 `*[]*struct`。

NULL 写成零值；字段是指针时写成 nil。非 NULL 扫进 `*T` 时会分配 `T`。接受 `sql.Scanner`、`driver.Valuer`、`bool`、整数、浮点、字符串和 `[]byte`。无符号整数写不进有符号字段时返回错误。nil 指针绑定为 SQL NULL。

`time.Time` 写成墙钟文本 `2006-01-02 15:04:05`。零值时间绑定为 NULL。可选的 `record.Flag` 和 `record.Decimal` 在 `github.com/fluxsce/dbx/record`。空 Flag 写成 `N`。空 Decimal 写成 NULL。

## 分页

`QueryPage`、`SelectPage` 和 `PageSQL` 使用当前引擎的分页。`Page` 的字段是 `Page`、`PageSize`、`OrderBy`、`Desc`。小于 1 的页码按 1。小于 1 的页大小按 20。dbx 不截断页大小，也不解析 URL。`OrderBy` 替换最外层 `ORDER BY`。各引擎子句见 [引擎](engines.zh-CN.md)。

## 一次发布可以改什么

补丁版修正绑定、扫描或引擎 SQL。函数签名和本页规则不变。`INSERT`、`UPSERT`、分页的具体文本，以及分批大小，可以改变。

次版本可以增加方法、可选方言接口、驱动和 `ErrorKind` 值。已有调用点保持可编译。新的 `ErrorKind` 在本页另行写明之前，`Retryable()` 为 false。

下列变更升主版本。导入路径改为 `github.com/fluxsce/dbx/v2`。

- 改变 `@name`，或在业务 SQL 中接受 `?` 和 `$1`。
- 把连接池和事务拆成两种会话类型。
- 允许空条件的 `Update` 或 `Delete`。
- 包装驱动错误，使 `errors.As` 看不到驱动类型。
- 改变 `db` 标签选项的含义。
- 把 [引擎](engines.zh-CN.md) 中某一行改成另一种 SQL 形态。

## 1.x 保持的限制

后续 1.x 可以在旁边增加可选能力，不会悄悄改掉这些限制。

- 没有保存点。嵌套 `Begin` 返回错误。嵌套 `Tx` 加入当前事务。
- `QueryRow` 成功时没有追踪，也没有插件事件。驱动调用要到 `Scan` 才发生。见 [观测](observability.zh-CN.md)。
- ClickHouse 没有 `Upsert`。调用会返回错误。
- ClickHouse 的更新和删除是 `ALTER TABLE`。已经执行的语句，ClickHouse 事务不会撤销。
- 扫描目标是带 `db` 标签的结构体。map 和标量切片不能作为扫描目标。
- 关联、迁移、软删除和查询构造器不在本模块内。SQL 由调用方编写。
- 多个数据库就是多个连接池。调用方持有 `map[string]*db.DB`，并逐个关闭。
- `SQL()` 返回底层 `*sql.DB`，供迁移和驱动特例使用。经它发出的语句不走 `@name`、追踪和插件。
