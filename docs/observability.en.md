# Observability

[中文](observability.zh-CN.md)

Statement logging is `Config.Trace`. Metrics, audit, and tracing are `Config.Plugins` on that pool. There is no process-wide plugin list. Hooks are copied onto transactions opened from the pool and close with the pool.

A hook must not call the same `*DB`. The connection may still be checked out. A hook must not panic.

`Args` on an event are the bound values, in order. They can contain passwords, tokens, and other secrets. Redact them in the trace or the plugin before writing the event anywhere.

## Trace

`TraceFunc` runs after each finished driver call and after begin, commit, and rollback. A nil trace does nothing. It does not take a plugin name, and it runs before `EventPlugin`.

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

The sample logs the statement and the duration. It leaves `Args` out. Thresholds and format stay in the caller.

| Field | Meaning |
|---|---|
| `Op` | `exec`, `query`, `begin`, `commit`, or `rollback` |
| `Query` | SQL sent to the driver, after `@name` rewrite. Empty for begin, commit, and rollback. |
| `Args` | Bound values, in order. Do not mutate the slice. Treat every value as sensitive. |
| `Duration` | Time until the driver call returns |
| `Err` | The driver or session error. Nil on success. |
| `Rows` | `RowsAffected` for a successful statement. `0` for a query, because the row count is known only after the caller finishes reading. |

`Commit` and `Rollback` pass `context.Background()` into the trace. The begin context is not kept on the transaction value.

## Plugins

A plugin implements `Plugin.Name`. The name is unique and non-empty inside one pool. The same value may also implement:

| Interface | When |
|---|---|
| `ContextPlugin.Before` | Before a connection is taken. Return a new context, or nil to keep the current one. Put a span on the returned context. Do not run SQL here. |
| `EventPlugin.OnEvent` | After the driver call returns. Same `Event` as `Trace`. |
| `io.Closer` | When the pool `Close`s, after the pool itself is closed. |

`Before` runs only when a connection is about to be used. A compile error is traced and does not call `Before`.

## QueryRow

`QueryRow` returns `*sql.Row` before the driver runs. Success is decided in `Scan`, so a successful `QueryRow` has no `Trace` and no `OnEvent`. A compile error or a finished transaction still reports an error event. Use `Query` when the span must cover a successful read, and close the rows.

## What a span can rely on

`ContextPlugin` sees the operation and the rewritten SQL before the driver call. `EventPlugin` sees the error and the duration after it. Query row counts are not on the event. Later work inside a hook needs its own connection source. Calling this `*DB` again from `OnEvent` can block on the connection the query has not returned.
