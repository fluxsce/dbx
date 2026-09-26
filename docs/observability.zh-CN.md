# 观测

[English](observability.en.md)

语句日志是 `Config.Trace`。指标、审计和链路追踪是挂在这条连接池上的 `Config.Plugins`。没有进程级插件表。钩子会复制到该池开出的事务上，并随连接池关闭。

钩子不得再调用同一个 `*DB`。此时连接可能还没归还。钩子不得 panic。

事件上的 `Args` 是按顺序绑定的值，其中可以有密码、令牌和其它机密。写入日志或上报之前，在 Trace 或插件里打码。

## Trace

`TraceFunc` 在每次结束的驱动调用之后，以及 begin、commit、rollback 之后执行。nil 表示不记录。它不占用插件名，并且先于 `EventPlugin`。

```go
d, err := db.Open(ctx, db.Config{
    Driver: "sqlite",
    DSN:    "app.db",
    Trace: func(ctx context.Context, e db.Event) {
        if e.Err != nil || e.Duration > 200*time.Millisecond {
            log.Printf("%s %s %s rows=%d err=%v", e.Op, e.Duration, e.Query, e.Rows, e.Err)
        }
    },
})
```

上面的例子记录语句和耗时，不记录 `Args`。阈值和格式留在调用方。

| 字段 | 含义 |
|---|---|
| `Op` | `exec`、`query`、`begin`、`commit` 或 `rollback` |
| `Query` | 发给驱动的 SQL，已经完成 `@name` 改写。begin、commit、rollback 为空。 |
| `Args` | 按顺序绑定的值。不要改这个切片。每个值都按机密处理。 |
| `Duration` | 驱动调用返回前经过的时间 |
| `Err` | 驱动或会话错误。成功时为 nil。 |
| `Rows` | 成功语句的 `RowsAffected`。查询为 `0`，因为调用方读完之前不知道行数。 |

`Commit` 和 `Rollback` 传给 Trace 的 context 是 `context.Background()`。事务值上不保留 begin 时的 context。

## 插件

插件实现 `Plugin.Name`。名字在一条连接池内唯一且非空。同一个值还可以实现：

| 接口 | 时机 |
|---|---|
| `ContextPlugin.Before` | 取连接之前。返回新的 context；返回 nil 则保持当前 context。链路 span 放在返回的 context 上。不要在这里执行 SQL。 |
| `EventPlugin.OnEvent` | 驱动调用返回之后。`Event` 与 `Trace` 相同。 |
| `io.Closer` | 连接池 `Close` 时，在池本身关闭之后。 |

`Before` 只在即将使用连接时调用。编译错误会进入 Trace，不会调用 `Before`。

## QueryRow

`QueryRow` 在驱动执行之前就返回 `*sql.Row`。成功与否要到 `Scan` 才知道，因此成功的 `QueryRow` 没有 `Trace`，也没有 `OnEvent`。编译错误或事务已结束仍会报告错误事件。成功的读也要进 span 时，使用 `Query` 并关闭结果集。

## span 可以依赖什么

`ContextPlugin` 在驱动调用前看到操作名和改写后的 SQL。`EventPlugin` 在调用后看到错误和耗时。查询行数不在事件上。钩子里的后续工作需要自己的连接来源。在 `OnEvent` 里再调用这个 `*DB`，可能堵在查询尚未归还的连接上。
