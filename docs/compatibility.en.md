# Compatibility

[中文](compatibility.zh-CN.md)

This page is the 1.x contract. A program written against it keeps compiling, and the session behavior below stays the same, through every 1.x release. Breaking it is a new major version. See [Versioning](versioning.en.md).

Engine SQL forms are in [Engines](engines.en.md). A patch may change the exact text of generated SQL. It keeps the form listed there.

## Stable behavior

- Business SQL uses `@name`. The key in `Args` has no `@`. `@@` is a literal `@`. A `?` placeholder in business SQL returns an error. Quotes, comments, and bracketed identifiers are left as written.
- `IN (@name)` expands a slice or array into one placeholder per element. An empty slice returns an error. `[]byte` is one value. A slice outside `IN` returns an error.
- A pool `*DB` commits each successful statement and may be used from many goroutines. A transaction `*DB` stays on one goroutine until `Commit` or `Rollback`.
- `Begin` on an open transaction returns an error. `Tx` on an open transaction runs the callback on that transaction. The outer caller commits or rolls back. Options passed to the inner `Tx` are ignored.
- `Tx` commits when the callback returns nil. It rolls back when the callback returns an error or panics, then it re-panics.
- `Update` and `Delete` require a `Cond` from `PK` or `Where`. An empty condition returns an error. `SET` and `WHERE` are bound separately, so one name can be the new value in the row and the old value in the condition.
- `Get`, `Select`, and `Each` close their rows. `Query` and `QueryPage` return rows for the caller to close. `Select` replaces the destination slice.
- Scanning matches column names to `db` tags. Extra columns are ignored. An exported field with no `db` tag is not a column. `db:"-"` skips a field.
- `Classify` returns an `ErrorKind` and leaves the driver error unwrapped. `errors.Is` and `errors.As` still see the original error. `Retryable` is true for deadlocks, lock waits, and serialization failures. A unique violation is not retryable.
- On MySQL and MariaDB, `Upsert` updates the row for whichever unique key was hit. The statement cannot name the key. On other engines that support `Upsert`, the key columns passed to the call are the conflict target.
- `OrderBy` and `EachTable` order lists accept identifiers only. dbx quotes them for the current engine.

## Struct tags

| Tag | Effect |
|---|---|
| `db:"name"` | Column name. An exported field without this tag is not a column. |
| `,pk` | Key column for `PK`. A composite key is several fields with `,pk`. |
| `,omitempty` | A zero value is left out of `INSERT` and `UPDATE`. |
| `,noupdate` | The column is inserted, and left out of `UPDATE` and of the `Upsert` update list. |
| `db:"-"` | The field is not a column. |

Anonymous embedded structs are flattened. Unexported fields are skipped.

`Insert`, `Update`, and `Upsert` accept `*struct`, `[]struct`, `[]*struct`, and `*[]struct`. `Get` and `Each` scan into `*struct`. `Select` and `SelectPage` scan into `*[]struct` or `*[]*struct`.

NULL becomes the zero value, or a nil pointer when the field is a pointer. A non-NULL value scanned into `*T` allocates `T`. `sql.Scanner`, `driver.Valuer`, `bool`, integers, floats, strings, and `[]byte` are accepted. An unsigned integer that does not fit in a signed field returns an error. A nil pointer binds as SQL NULL.

`time.Time` is written as wall-clock text `2006-01-02 15:04:05`. A zero time binds as NULL. Optional `record.Flag` and `record.Decimal` live in `github.com/fluxsce/dbx/record`. An empty Flag is written as `N`. An empty Decimal is written as NULL.

## Paging

`QueryPage`, `SelectPage`, and `PageSQL` apply the current engine's paging. `Page` fields are `Page`, `PageSize`, `OrderBy`, and `Desc`. A page below 1 becomes 1. A page size below 1 becomes 20. dbx does not cap the page size and does not read the URL. `OrderBy` replaces the outer `ORDER BY`. Per-engine clauses are in [Engines](engines.en.md).

## What a release may change

A patch fixes binding, scanning, or engine SQL. Signatures and the rules on this page stay. The exact text of `INSERT`, `UPSERT`, paging, and batch sizes may change.

A minor release may add methods, optional dialect interfaces, drivers, and `ErrorKind` values. Existing call sites keep compiling. A new `ErrorKind` has `Retryable() == false` until this page says otherwise.

The following are a new major version. The import path becomes `github.com/fluxsce/dbx/v2`.

- Changing `@name`, or accepting `?` and `$1` in business SQL.
- Splitting the pool and the transaction into two session types.
- Allowing `Update` or `Delete` with an empty condition.
- Wrapping driver errors so `errors.As` no longer sees the driver type.
- Changing the meaning of `db` tag options.
- Changing an engine's row in [Engines](engines.en.md) to a different SQL form.

## Limits that 1.x keeps

A later 1.x may add an opt-in beside these. It does not silently change them.

- There is no savepoint. Nested `Begin` returns an error. Nested `Tx` joins the current transaction.
- `QueryRow` does not emit a success trace or plugin event. The driver call waits until `Scan`. See [Observability](observability.en.md).
- `Upsert` is absent on ClickHouse. Calling it returns an error.
- ClickHouse updates and deletes are `ALTER TABLE` mutations. A ClickHouse transaction does not undo a statement that already ran.
- Scan destinations are structs with `db` tags. Maps and scalar slices are not destinations.
- Associations, migrations, soft delete, and a query builder are outside this module. Hand-written SQL stays in the caller.
- Several databases are several pools. The caller owns `map[string]*db.DB` and closes each pool.
- `SQL()` returns the underlying `*sql.DB` for migrations and driver-specific work. Statements issued through it skip `@name`, traces, and plugins.
