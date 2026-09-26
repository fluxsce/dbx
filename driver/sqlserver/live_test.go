package sqlserver_test

import (
	"os"
	"testing"

	_ "github.com/fluxsce/dbx/driver/sqlserver"
	"github.com/fluxsce/dbx/internal/livetest"
)

// 真库场景覆盖 SQL Server 与 SQLite 不同的部分：业务 SQL 仍写 @name，驱动把 ? 收成 @p1；
// 标识符用方括号；分页是 OFFSET/FETCH；Upsert 是 MERGE；2627 收成唯一冲突。
// 设置 DBX_SQLSERVER_DSN 才运行。
// 例：sqlserver://sa:Dbx_ci_Pass1@127.0.0.1:1433?database=master&encrypt=disable
// 未设置时跳过，默认 go test 不连接这台机器。
func TestLiveSQLServer(t *testing.T) {
	dsn := os.Getenv("DBX_SQLSERVER_DSN")
	if dsn == "" {
		t.Skip("DBX_SQLSERVER_DSN is empty")
	}
	livetest.Run(t, "sqlserver", dsn, "dbx_it_mssql", "DECIMAL(12,2)", "DATETIME2", "BIT")
}
