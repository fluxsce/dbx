# dbx

[中文说明](README.zh-CN.md) · [Repository](https://github.com/fluxsce/dbx) · [Changelog](CHANGELOG.md) · [License](LICENSE)

A Go database session: one `*DB`, named parameters `@name`, and one dialect plugin per engine. The pool and a transaction are the same type.

## Install

```bash
go get github.com/fluxsce/dbx@v1.0.0
```

The module path is `github.com/fluxsce/dbx`. v1 has no `/v2` suffix. Blank-import the drivers you link.

## Sessions

A pool `*DB` auto-commits each successful statement. Many goroutines may share that pool.

`Begin` returns a transaction the caller commits or rolls back. That value stays on one goroutine, and only until `Commit` or `Rollback`. `Begin` inside an open transaction returns an error.

`Tx` commits when the callback returns nil, and rolls back when it returns an error or panics. The panic continues after rollback. A `Tx` call inside an open transaction joins that transaction.

Each `Begin` on the pool returns its own transaction. Goroutines keep their own transaction values.

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

err = d.TxOptions(ctx, &sql.TxOptions{ReadOnly: true}, func(tx *db.DB) error {
    return tx.Each(ctx, &row, `SELECT id, name FROM users ORDER BY id`, nil, writeRow)
})
```

SQL uses `@name` only. Keys in `Args` have no `@`. The dialect rewrites that SQL to `?`, `$1`, or `:1`. A slice passed to `IN (@ids)` expands into one placeholder per element. An empty slice is an error. `[]byte` stays one value. A slice outside `IN` is an error. DDL goes through `Exec`.

`Upsert` inserts a row when its key is absent and updates the other columns when the key exists. It is a separate method from `Insert`. ClickHouse does not support it. `Classify` maps unique violations, deadlocks, lock waits, and serialization failures to one kind. Retry the whole transaction only when `Retryable` is true. A unique violation is not retried as-is.

`BeginOptions` and `TxOptions` take `*sql.TxOptions`. A nil value uses the engine default isolation level.

One `Open` is one pool. An application with several databases keeps its own `map[string]*db.DB` and closes each pool when the process shuts down.

`Config.Trace` is the built-in statement log. It runs after each statement and after begin, commit, and rollback, and it does not take a plugin name. A nil trace does nothing. `Config.Plugins` are bound to that pool, in order, for metrics, audit, or tracing. A `ContextPlugin` may replace the context before a connection is taken. Slow-query thresholds and log formatting stay in the caller. Plugins close with the pool. A hook must not call back into the same `*DB`.

`Get`, `Select`, and `Each` close their rows. `Query` and `QueryPage` return rows for the caller to close.

## Engines

Change `Driver` and `DSN`, and blank-import that driver.

| Import | Names |
|---|---|
| `github.com/fluxsce/dbx/driver/sqlite` | `sqlite`, `sqlite3` |
| `github.com/fluxsce/dbx/driver/postgres` | `postgres`, `postgresql`, `pg` |
| `github.com/fluxsce/dbx/driver/mysql` | `mysql`, `mariadb` |
| `github.com/fluxsce/dbx/driver/clickhouse` | `clickhouse` |
| `github.com/fluxsce/dbx/driver/sqlserver` | `sqlserver`, `mssql` |
| `github.com/fluxsce/dbx/driver/oracle` | `oracle`, `godror`, `oracle11g` (build with `-tags oracle`) |

ClickHouse is pure Go and is linked only when imported. Oracle needs CGO, Oracle Instant Client, and `-tags oracle`. The default `go test ./...` does not compile godror.

A new engine implements `db.Dialect` (`Name`, `Rebind`, `QuoteIdent`, `LimitSQL`) and registers an opener with `db.Register`. Placeholder rebinding and identifier quotes live in `utils`. Sessions, transactions, and struct mapping stay in `db`.

ClickHouse updates and deletes are `ALTER TABLE`. SQL Server keeps `?`; its driver turns that into `@p1`. Oracle 12c uses `:1` and `FETCH`. `Driver: "oracle11g"` rewrites the statement with `ROW_NUMBER`.

`Insert` and `Update` quote identifiers for the current engine. Hand-written SQL should use `d.QuoteColumns`.

## Paging

Use `QueryPage`, `SelectPage`, or `PageSQL`. The limit clause is written by the current dialect. `Page` fields are `page`, `pageSize`, `OrderBy`, and `Desc`. A `pageSize` below 1 becomes 20. dbx does not cap the size and does not read the URL.

`OrderBy` replaces the outer `ORDER BY`. `Desc` selects descending order. A `FETCH` query already needs `ORDER BY` when `OrderBy` is empty.

## Columns

`github.com/fluxsce/dbx/record` is optional.

| Column | Go | Behavior |
|---|---|---|
| Y/N flag | `record.Flag` | empty Flag is written as `N` |
| DECIMAL / money | `record.Decimal` | empty Decimal is written as NULL |
| DATETIME text | `time.Time` | wall clock `2006-01-02 15:04:05`; zero is NULL |

Scanning matches columns to `db` tags. It accepts structs, `*struct`, `[]struct`, `[]*struct`, and anonymous embeds. A non-NULL `*T` is allocated. NULL becomes the zero value or a nil pointer. `sql.Scanner` (including `sql.Null*`), `bool`, integers, floats, strings, and `[]byte` are accepted. An unsigned value that does not fit in a signed field returns an error. Slice and map elements are converted when their types differ.

Oracle `NUMBER` and LOB values are read in `driver/oracle`. ClickHouse UUID, Decimal, big integers, and Geo values are read in `driver/clickhouse`. Arrays and maps use the shared slice and map assignment.

Struct tags: `db:"name"`; composite key `,pk`; skip `db:"-"`; omit a zero `,omitempty`; leave a column out of UPDATE `,noupdate`.

`Update` and `Delete` require a `Cond` (`d.PK` or `db.Where`). An empty condition is an error.

## Layout

```text
dbx/
  db/                 # *DB: pool, SQL, transactions, insert/update/delete
  utils/              # placeholder rebind and identifier quoting
  record/             # optional column types: Flag, Decimal, wall-clock time
  driver/             # sqlite, postgres, mysql, clickhouse, sqlserver, oracle
```

## Project

Source: <https://github.com/fluxsce/dbx>

Versions start at 1.0.0. A push to `main` that passes tests reads the first `## [x.y.z] - date` heading in [CHANGELOG.md](CHANGELOG.md), pushes tag `vX.Y.Z` when that tag is absent, and creates a GitHub Release. `go get` fetches that tag from this public repository. The module proxy caches it on the first request.

Apache License 2.0. See [LICENSE](LICENSE).
