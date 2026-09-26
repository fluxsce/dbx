# Engines

[中文](engines.zh-CN.md)

Business SQL always uses `@name`. The columns below are what the driver receives after rebind. A patch may change whitespace and clause order. It keeps the form in this table. Batch row caps and parameter caps may change in a patch.

| Engine | Driver placeholder | Identifiers | Paging | Upsert | Update / Delete |
|---|---|---|---|---|---|
| SQLite | `?` | `"` | `LIMIT` / `OFFSET` | `ON CONFLICT` | `UPDATE` / `DELETE` |
| PostgreSQL | `$1` | `"` | `LIMIT` / `OFFSET` | `ON CONFLICT` | `UPDATE` / `DELETE` |
| MySQL, MariaDB | `?` | `` ` `` | `LIMIT` / `OFFSET` | `ON DUPLICATE KEY UPDATE` | `UPDATE` / `DELETE` |
| SQL Server | `?`, then the driver uses `@p1` | `[` `]` | `OFFSET` / `FETCH` | `MERGE` | `UPDATE` / `DELETE` |
| Oracle 12c and newer | `:1` | `"` | `OFFSET` / `FETCH` | `MERGE` | `UPDATE` / `DELETE` |
| Oracle 11g | `:1` | `"` | `ROW_NUMBER` around the statement | `MERGE` | `UPDATE` / `DELETE` |
| ClickHouse | `?` | `` ` `` | `LIMIT` / `OFFSET` | unsupported | `ALTER TABLE … UPDATE` / `DELETE` |

`Driver: "oracle11g"` selects the Oracle 11g row. `oracle` and `godror` select Oracle 12c.

`OFFSET` / `FETCH` needs an `ORDER BY`. Pass `Page.OrderBy`, or put `ORDER BY` in the statement.

MySQL and MariaDB update whichever unique key was hit. The call's key names are checked against the struct, and they are not written into the statement.

ClickHouse has no `Upsert`. Its updates and deletes are mutations. A transaction does not undo a statement that already ran. Multi-row `Insert` on the pool uses the native batch.

Oracle needs CGO, Oracle Instant Client, and `-tags oracle`. The default `go test ./...` does not compile that driver. ClickHouse is pure Go and is linked only when imported.

A new engine implements `db.Dialect` (`Name`, `Rebind`, `QuoteIdent`, `LimitSQL`) and registers an opener with `db.Register`. Optional interfaces cover mutations, whole-statement paging, batch limits, bulk insert, upsert, error classification, and scan conversion. Session code stays in `db`.
