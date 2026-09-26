# dbx

[English](README.md) · [文档](docs/compatibility.zh-CN.md) · [变更记录](CHANGELOG.md) · [许可证](LICENSE)

基于 `database/sql` 的 Go 数据库会话。业务 SQL 使用 `@name`。一个 `*DB` 同时是连接池和事务。每种引擎是一个方言插件。

```bash
go get github.com/fluxsce/dbx@latest
```

发布物是 Git 标签。`go.mod` 里的 `go` 行是 Go 语言版本。见 [版本](docs/versioning.zh-CN.md)。

只空白导入要链接的驱动。只导入 `db` 不会链进任何引擎。

## 示例

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

连接池上的 `*DB` 在每条成功语句后提交，可以被多个 goroutine 共用。`Begin` 返回的事务由调用方提交或回滚，并且只在一个 goroutine 里使用。`Tx` 在回调返回 nil 时提交，返回错误或 panic 时回滚。

SQL 写 `@name`，`Args` 的键不带 `@`。方言把占位符改成 `?`、`$1` 或 `:1`。`IN (@ids)` 展开切片。`Update` 和 `Delete` 必须带条件。

标签、分页和错误见 [兼容性](docs/compatibility.zh-CN.md)。各引擎的 SQL 见 [引擎](docs/engines.zh-CN.md)。

## 引擎

更换 `Driver` 和 `DSN`，并空白导入对应驱动。

| 导入 | 名称 |
|---|---|
| `github.com/fluxsce/dbx/driver/sqlite` | `sqlite`、`sqlite3` |
| `github.com/fluxsce/dbx/driver/postgres` | `postgres`、`postgresql`、`pg` |
| `github.com/fluxsce/dbx/driver/mysql` | `mysql`、`mariadb` |
| `github.com/fluxsce/dbx/driver/clickhouse` | `clickhouse` |
| `github.com/fluxsce/dbx/driver/sqlserver` | `sqlserver`、`mssql` |
| `github.com/fluxsce/dbx/driver/oracle` | `oracle`、`godror`、`oracle11g`（编译加 `-tags oracle`） |

ClickHouse 是纯 Go，只有导入之后才会链进程序。Oracle 需要 CGO、Oracle Instant Client 和 `-tags oracle`。`go test ./...` 不编译 godror。

新引擎实现 `db.Dialect`，并用 `db.Register` 登记打开函数。会话、事务和结构体映射留在 `db`。

## 文档

| | |
|---|---|
| [兼容性](docs/compatibility.zh-CN.md) | 1.x 内保持的行为 |
| [引擎](docs/engines.zh-CN.md) | 占位符、分页、upsert 和变更语句 |
| [版本](docs/versioning.zh-CN.md) | 模块标签、`go` 行和发布 |
| [从 sqlx 迁入](docs/from-sqlx.zh-CN.md) | 占位符、会话和扫描的差异 |
| [观测](docs/observability.zh-CN.md) | `Trace`、插件，以及 `QueryRow` 的限制 |
| [English](README.md) | The same pages in English |

Apache License 2.0。见 [LICENSE](LICENSE)。
