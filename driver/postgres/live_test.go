package postgres_test

import (
	"os"
	"testing"

	_ "github.com/fluxsce/dbx/driver/postgres"
	"github.com/fluxsce/dbx/internal/livetest"
)

// 真库场景覆盖 PostgreSQL 与 SQLite 不同的部分：$n 占位符、双引号保留大小写列名、
// LIMIT/OFFSET、ON CONFLICT，以及 23505 收成唯一冲突。
// 设置 DBX_POSTGRES_DSN 才运行。例：postgres://postgres:postgres@127.0.0.1:5432/dbx?sslmode=disable
// 未设置时跳过，默认 go test 不连接这台机器。
func TestLivePostgres(t *testing.T) {
	dsn := os.Getenv("DBX_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DBX_POSTGRES_DSN is empty")
	}
	livetest.Run(t, "postgres", dsn, "dbx_it_pg", "NUMERIC(12,2)", "TIMESTAMP", "BOOLEAN")
}
