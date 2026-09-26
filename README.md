# dbx

[中文说明](README.zh-CN.md)

`github.com/fluxsce/dbx` is a small database session for Go: one `*DB`, named SQL parameters, and a dialect plugin per engine.

Remote: `https://github.com/fluxsce/dbx.git`

It follows `database/sql` (core plus registered drivers) and the GORM idea that one type is both the pool and a transaction.

```text
dbx/
  db/                 # *DB: pool, SQL, transactions, insert/update/delete
  utils/              # placeholder rebind and identifier quoting
  record/             # optional column types: Flag, Decimal, wall-clock time
  driver/
    sqlite/
    postgres/
    mysql/
    clickhouse/
    sqlserver/
    oracle/           # not linked unless built with -tags oracle; registers oracle and oracle11g
```

SQL uses `@name` only. Args keys have no `@`. Do not write `?` or `$1`. DDL goes through `Exec`.

## Sessions

A pool `*DB` auto-commits each successful statement. Many goroutines may share that pool.

`Begin` returns a transaction the caller commits or rolls back. That value is one goroutine only. Do not pass it to another goroutine, and do not use it after `Commit` or `Rollback`.

`Tx` is the same transaction with the commit decision in the callback: nil commits, an error or panic rolls back. A `Tx` started inside an open transaction joins that transaction.

Several goroutines may each call `Begin` on the same pool. Each returned transaction is independent. They must not share one transaction value.

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

// Auto-commit: this INSERT is committed when Exec returns nil.
_, err = d.Exec(ctx, `
    INSERT INTO users (id, name) VALUES (@id, @name)`,
    db.Args{"id": "u1", "name": "ada"},
)

// Caller commits.
tx, err := d.Begin(ctx)
if _, err = tx.Exec(ctx, `UPDATE users SET name=@name WHERE id=@id`, db.Args{"id": "u1", "name": "ada lovelace"}); err != nil {
    _ = tx.Rollback()
    return err
}
if err = tx.Commit(); err != nil {
    return err
}

// Callback commits on nil and rolls back on error.
err = d.Tx(ctx, func(tx *db.DB) error {
    return tx.Insert(ctx, "users", &row)
})

err = d.TxOptions(ctx, &sql.TxOptions{ReadOnly: true}, func(tx *db.DB) error {
    return tx.Each(ctx, &row, `SELECT id, name FROM users ORDER BY id`, nil, writeRow)
})
```

`BeginOptions` and `TxOptions` take `*sql.TxOptions` for isolation and read-only. A nil options value uses the engine default.

One `Open` is one pool. An application that talks to several databases keeps its own `map[string]*db.DB` and closes each pool when that process shuts down. dbx does not keep a global connection cache.

`Config.Trace` runs after each statement and after begin, commit, and rollback. A nil trace does nothing. Slow-query thresholds and log formatting stay in the caller.

## Engines

Change `Driver` and `DSN`. Blank-import only the drivers you link.

| Import | Names |
|---|---|
| `github.com/fluxsce/dbx/driver/sqlite` | `sqlite`, `sqlite3` |
| `github.com/fluxsce/dbx/driver/postgres` | `postgres`, `postgresql`, `pg` |
| `github.com/fluxsce/dbx/driver/mysql` | `mysql`, `mariadb` |
| `github.com/fluxsce/dbx/driver/clickhouse` | `clickhouse` |
| `github.com/fluxsce/dbx/driver/sqlserver` | `sqlserver`, `mssql` |
| `github.com/fluxsce/dbx/driver/oracle` | `oracle`, `godror`, `oracle11g`（build with `-tags oracle`） |

A new engine implements `db.Dialect` (`Name`, `Rebind`, `QuoteIdent`, `LimitSQL`) and registers an opener with `db.Register`. The limit clause lives in that driver. Oracle 11g (`Driver: "oracle11g"`) implements `PageDialect` and rewrites the SELECT with `ROW_NUMBER`. Use `QueryPage`, `SelectPage`, or `PageSQL` for paging. `Page.OrderBy` replaces the outer `ORDER BY`; `Desc` selects descending order. A `FETCH` query must already have `ORDER BY` when `OrderBy` is empty.

Page with `QueryPage` / `SelectPage`. Do not hard-code `LIMIT` in business SQL. `Page` uses `page` and `pageSize`. A size below 1 becomes 20. dbx does not cap the size and does not read the URL.

`Insert` / `Update` quote identifiers for the current engine. Hand-written SQL should use `d.QuoteColumns`.

## Column helpers

`github.com/fluxsce/dbx/record` is optional:

| Column | Go | Behavior |
|---|---|---|
| Y/N flag | `record.Flag` | empty Flag is written as `N` |
| DECIMAL / money | `record.Decimal` | empty Decimal is written as NULL |
| DATETIME text | `time.Time` | wall clock `2006-01-02 15:04:05`; zero is NULL |

Scanning also accepts unsigned integers into signed fields when the value fits, and converts slice and map elements. Oracle `NUMBER` and LOB values are read in `driver/oracle`. ClickHouse UUID, Decimal, big integers, and Geo values are read in `driver/clickhouse`. Arrays and maps use the shared slice and map assignment.

Struct tags: `db:"name"`; composite key `,pk`; skip `db:"-"`; omit a zero `,omitempty`; leave a column out of UPDATE `,noupdate`.

`Update` and `Delete` require a `Cond` (`d.PK` or `db.Where`). An empty condition is an error.

## Release

Versions start at 1.0.0. The module path stays `github.com/fluxsce/dbx` (v1 does not use a `/v2` suffix). The Git remote is `https://github.com/fluxsce/dbx.git`. Pushing `main` runs tests, then tags `vX.Y.Z` from the first version heading in [CHANGELOG.md](CHANGELOG.md) and publishes a GitHub Release when that tag is not already present. See [CHANGELOG.md](CHANGELOG.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).
