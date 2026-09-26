# From sqlx

[中文](from-sqlx.zh-CN.md)

dbx covers the same jobs as sqlx: named SQL, struct scan, `IN` lists, placeholder rebind, and transactions. Single-table `Insert`, `Update`, `Delete`, `Upsert`, paging, and `Classify` are additional. Engine forms are in [Engines](engines.en.md).

## Placeholders

sqlx accepts `?`, `$1`, or `:name`, depending on the bind type. dbx accepts `@name` only. The key has no `@`.

```go
// sqlx
db.NamedExec(`INSERT INTO users (id, name) VALUES (:id, :name)`,
    map[string]any{"id": "u1", "name": "ada"})

// dbx
_, err = d.Exec(ctx, `INSERT INTO users (id, name) VALUES (@id, @name)`,
    db.Args{"id": "u1", "name": "ada"})
```

`@@` writes a literal `@`. A `?` in business SQL is an error, including SQL that used to rely on `Rebind`.

## Sessions

sqlx uses `*sqlx.DB` and `*sqlx.Tx`. dbx uses `*db.DB` for both. The methods are the same on either value.

```go
err = d.Tx(ctx, func(tx *db.DB) error {
    _, err := tx.Exec(ctx, `UPDATE users SET name = @name WHERE id = @id`,
        db.Args{"id": "u1", "name": "ada lovelace"})
    return err
})
```

`Begin` inside a transaction returns an error. `Tx` inside a transaction joins it. There is no savepoint API. A pool may be shared. A transaction value stays on one goroutine.

## Scanning

`Get` and `Select` match columns to `db` tags. Each column needs a tag. An exported field with no tag is not a column.

```go
type User struct {
    ID   string `db:"id,pk"`
    Name string `db:"name"`
}
```

Destinations are structs: `*struct`, `*[]struct`, and `*[]*struct`. `map[string]any` and `[]string` are not scan destinations. `Each` streams rows into one struct. The callback copies the struct if it keeps the value. `Select` replaces the slice.

## `IN`

```go
err = d.Select(ctx, &users, `SELECT id, name FROM users WHERE id IN (@ids)`,
    db.Args{"ids": []string{"u1", "u2"}})
```

An empty slice is an error. Write another statement when the list can be empty. `[]byte` stays a single parameter.

## Writes

```go
func save(ctx context.Context, d *db.DB, user *User) error {
    if err := d.Insert(ctx, "users", user); err != nil {
        return err
    }
    cond, err := d.PK(user)
    if err != nil {
        return err
    }
    user.Name = "ada lovelace"
    return d.Update(ctx, "users", user, cond)
}
```

`PK` reads fields tagged `,pk`. `Where` takes a fragment with `@name` and no `WHERE` keyword. An empty condition is an error.

`Upsert(ctx, table, row, keyColumns...)` inserts or updates in one statement. The key column order follows the unique index. ClickHouse returns an error. MySQL and MariaDB update whichever unique key was hit.

## Errors

`Classify` maps a driver error to `unique`, `deadlock`, `lock`, `serialization`, or `other`. Retry the whole transaction when `Retryable()` is true. A unique violation stays a failure until the data changes. The returned error is still the driver error.
