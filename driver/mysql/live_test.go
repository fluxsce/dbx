package mysql_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/mysql"
	"github.com/fluxsce/dbx/record"
)

// 真库场景覆盖 MySQL 与 SQLite 不同的类型和分页。
// 设置 DBX_MYSQL_DSN 才运行。驱动以最后一个 @ 分隔地址，口令里的 @ 可以原样放在用户名后面。
// 例：root:p@ss@tcp(127.0.0.1:3306)/db?parseTime=true
// 未设置时跳过，默认 go test 不连接这台机器。
func TestLiveMySQL(t *testing.T) {
	dsn := os.Getenv("DBX_MYSQL_DSN")
	if dsn == "" {
		t.Skip("DBX_MYSQL_DSN is empty")
	}
	if !strings.Contains(dsn, "parseTime=") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}

	var events []db.Event
	d, err := db.Open(context.Background(), db.Config{
		Driver: "mysql",
		DSN:    dsn,
		Trace: func(_ context.Context, e db.Event) {
			events = append(events, e)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	ctx := context.Background()
	table := "dbx_it_live"
	if _, err := d.Exec(ctx, "DROP TABLE IF EXISTS "+table, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Exec(context.Background(), "DROP TABLE IF EXISTS "+table, nil)
	})
	ddl := `CREATE TABLE ` + table + ` (
		name VARCHAR(64) NOT NULL,
		amount DECIMAL(12,2) NULL,
		on_flag CHAR(1) NOT NULL,
		at DATETIME NULL,
		yes TINYINT NOT NULL
	)`
	if _, err := d.Exec(ctx, ddl, nil); err != nil {
		t.Fatal(err)
	}

	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	type row struct {
		Name   string         `db:"name"`
		Amount record.Decimal `db:"amount"`
		Flag   record.Flag    `db:"on_flag"`
		At     *time.Time     `db:"at"`
		Yes    bool           `db:"yes"`
	}
	if err := d.Insert(ctx, table, &row{
		Name: "ada", Amount: "12.50", Flag: record.FlagY, At: &when, Yes: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Insert(ctx, table, &row{
		Name: "empty-time", Amount: "", Flag: "", Yes: false,
	}); err != nil {
		t.Fatal(err)
	}

	var got row
	q := `SELECT name, amount, on_flag, at, yes FROM ` + table + ` WHERE name=@name`
	if err := d.Get(ctx, &got, q, db.Args{"name": "ada"}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "ada" || got.Amount != "12.50" || got.Flag != record.FlagY || !got.Yes || got.At == nil {
		t.Fatalf("%+v", got)
	}
	if got.At.Format("2006-01-02 15:04:05") != "2026-01-02 03:04:05" {
		t.Fatalf("at %s", got.At)
	}
	var empty row
	if err := d.Get(ctx, &empty, q, db.Args{"name": "empty-time"}); err != nil {
		t.Fatal(err)
	}
	if empty.At != nil || empty.Flag != record.FlagN || empty.Yes || empty.Amount != "" {
		t.Fatalf("nulls %+v", empty)
	}

	err = d.Tx(ctx, func(tx *db.DB) error {
		_, err := tx.Exec(ctx, `INSERT INTO `+table+` (name, amount, on_flag, yes) VALUES (@name, @amount, @flag, @yes)`, db.Args{
			"name": "kept", "amount": "3.00", "flag": "Y", "yes": 1,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.Tx(ctx, func(tx *db.DB) error {
		_, err := tx.Exec(ctx, `INSERT INTO `+table+` (name, amount, on_flag, yes) VALUES (@name, @amount, @flag, @yes)`, db.Args{
			"name": "rolled", "amount": "1.00", "flag": "N", "yes": 0,
		})
		if err != nil {
			return err
		}
		return errForce
	})
	if err == nil {
		t.Fatal("expected rollback")
	}
	if err := d.Get(ctx, &got, q, db.Args{"name": "rolled"}); err != sql.ErrNoRows {
		t.Fatalf("rolled back row: %v", err)
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic must propagate")
			}
		}()
		_ = d.Tx(ctx, func(tx *db.DB) error {
			if _, err := tx.Exec(ctx, `INSERT INTO `+table+` (name, amount, on_flag, yes) VALUES (@name, @amount, @flag, @yes)`, db.Args{
				"name": "boom", "amount": "1.00", "flag": "N", "yes": 0,
			}); err != nil {
				return err
			}
			panic("dbx-test")
		})
	}()
	if err := d.Get(ctx, &got, q, db.Args{"name": "boom"}); err != sql.ErrNoRows {
		t.Fatalf("panic row: %v", err)
	}
	if _, err := d.Exec(ctx, `INSERT INTO `+table+` (name, amount, on_flag, yes) VALUES (@name, @amount, @flag, @yes)`, db.Args{
		"name": "after", "amount": "2.00", "flag": "Y", "yes": 1,
	}); err != nil {
		t.Fatal(err)
	}

	var page []row
	total, err := d.SelectPage(ctx, &page,
		`SELECT COUNT(*) FROM `+table,
		`SELECT name, amount, on_flag, at, yes FROM `+table+` ORDER BY name`,
		db.Page{Page: 1, PageSize: 1, OrderBy: "name"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(page) != 1 || page[0].Name != "ada" {
		t.Fatalf("page total=%d rows=%v", total, page)
	}

	var sawBegin, sawCommit, sawRollback bool
	for _, e := range events {
		switch e.Op {
		case "begin":
			sawBegin = true
		case "commit":
			sawCommit = true
			if e.Err != nil {
				t.Fatal(e.Err)
			}
		case "rollback":
			sawRollback = true
		}
	}
	if !sawBegin || !sawCommit || !sawRollback {
		t.Fatalf("trace begin=%v commit=%v rollback=%v", sawBegin, sawCommit, sawRollback)
	}
}

var errForce = errString("force rollback")

type errString string

func (e errString) Error() string { return string(e) }
