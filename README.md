# dbx

[中文](README.zh-CN.md) · [Docs](docs/compatibility.en.md) · [Changelog](CHANGELOG.md) · [License](LICENSE)

A Go database session on top of `database/sql`. You write SQL with `@name` parameters. One `*DB` is both the pool and a transaction. Each engine is a dialect plugin.

```bash
go get github.com/fluxsce/dbx@latest
```

Releases are Git tags. The `go` line in `go.mod` is the Go language version. See [Versioning](docs/versioning.en.md).

Blank-import only the drivers you link. Importing `db` does not link an engine.

## Example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/sqlite"
)

type User struct {
	ID   string `db:"id,pk"`
	Name string `db:"name"`
}

func main() {
	ctx := context.Background()
	d, err := db.Open(ctx, db.Config{Driver: "sqlite", DSN: "app.db"})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	_, err = d.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id   TEXT PRIMARY KEY,
			name TEXT NOT NULL
		)`, nil)
	if err != nil {
		log.Fatal(err)
	}

	row := User{ID: "u1", Name: "ada"}
	if err = d.Insert(ctx, "users", &row); err != nil {
		log.Fatal(err)
	}

	var got User
	err = d.Get(ctx, &got, `SELECT id, name FROM users WHERE id = @id`, db.Args{"id": row.ID})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(got.Name)
}
```

A pool `*DB` commits each successful statement and may be shared across goroutines. `Begin` returns a transaction the caller commits or rolls back, on one goroutine. `Tx` commits when the callback returns nil, and rolls back when it returns an error or panics.

SQL uses `@name`. Keys in `Args` have no `@`. The dialect rewrites placeholders to `?`, `$1`, or `:1`. `IN (@ids)` expands a slice. `Update` and `Delete` require a condition.

Tags, paging, and errors are specified in [Compatibility](docs/compatibility.en.md). Per-engine SQL is in [Engines](docs/engines.en.md).

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

ClickHouse is pure Go and is linked only when imported. Oracle needs CGO, Oracle Instant Client, and `-tags oracle`. `go test ./...` does not compile godror.

A new engine implements `db.Dialect` and registers an opener with `db.Register`. Session, transaction, and struct mapping stay in `db`.

## Documentation

| | |
|---|---|
| [Compatibility](docs/compatibility.en.md) | Behavior that stays through 1.x |
| [Engines](docs/engines.en.md) | Placeholder, paging, upsert, and mutations |
| [Versioning](docs/versioning.en.md) | Module tags, the `go` line, and releases |
| [From sqlx](docs/from-sqlx.en.md) | Placeholder, session, and scan differences |
| [Observability](docs/observability.en.md) | `Trace`, plugins, and the `QueryRow` limit |
| [中文](README.zh-CN.md) | 同一套说明的中文版 |

Apache License 2.0. See [LICENSE](LICENSE).
