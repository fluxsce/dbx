package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/sqlite"
)

func TestOpenSQLiteNamedQuery(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (tenantId TEXT, demoName TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(ctx, `INSERT INTO AI_DEMO (tenantId, demoName) VALUES (@tenantId, @demoName)`, db.Args{
		"tenantId": "default",
		"demoName": "n1",
	}); err != nil {
		t.Fatal(err)
	}
	row, err := d.QueryRow(ctx, `SELECT demoName FROM AI_DEMO WHERE tenantId = @tenantId`, db.Args{"tenantId": "default"})
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := row.Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "n1" {
		t.Fatalf("got %q", name)
	}
}

func TestTraceExec(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	var events []db.Event
	d, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
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
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (demoName TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(ctx, `INSERT INTO AI_DEMO (demoName) VALUES (@demoName)`, db.Args{"demoName": "n1"}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Op != "exec" || events[1].Err != nil || events[1].Rows != 1 {
		t.Fatalf("%+v", events)
	}
}

func TestOpenAppliesPool(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{
		Driver:       "sqlite",
		DSN:          dsn,
		MaxOpenConns: 7,
		MaxIdleConns: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if n := d.SQL().Stats().MaxOpenConnections; n != 7 {
		t.Fatalf("MaxOpenConnections=%d", n)
	}
}

func TestTxCommitAndRollback(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (tenantId TEXT, demoName TEXT)`, nil); err != nil {
		t.Fatal(err)
	}

	err = d.Tx(ctx, func(tx *db.DB) error {
		_, err := tx.Exec(ctx, `INSERT INTO AI_DEMO (tenantId, demoName) VALUES (@tenantId, @demoName)`, db.Args{
			"tenantId": "default", "demoName": "ok",
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.Tx(ctx, func(tx *db.DB) error {
		if _, err := tx.Exec(ctx, `INSERT INTO AI_DEMO (tenantId, demoName) VALUES (@tenantId, @demoName)`, db.Args{
			"tenantId": "default", "demoName": "no",
		}); err != nil {
			return err
		}
		return fmt.Errorf("force rollback")
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}

	rows, err := d.Query(ctx, `SELECT demoName FROM AI_DEMO WHERE tenantId = @tenantId ORDER BY demoName`, db.Args{"tenantId": "default"})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if len(names) != 1 || names[0] != "ok" {
		t.Fatalf("autocommit+tx: got %v", names)
	}
}

func TestTxOptionsNestedJoins(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (tenantId TEXT, demoName TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	if d.LimitSQL() != "LIMIT @limit OFFSET @offset" {
		t.Fatalf("sqlite LimitSQL %q", d.LimitSQL())
	}

	err = d.TxOptions(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, func(outer *db.DB) error {
		if !outer.InTx() {
			t.Fatal("outer")
		}
		return outer.TxOptions(ctx, &sql.TxOptions{ReadOnly: true}, func(inner *db.DB) error {
			if !inner.InTx() || inner != outer {
				t.Fatal("must join current tx")
			}
			_, err := inner.Exec(ctx, `INSERT INTO AI_DEMO (tenantId, demoName) VALUES (@tenantId, @demoName)`, db.Args{
				"tenantId": "default", "demoName": "join",
			})
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := d.QueryRow(ctx, `SELECT demoName FROM AI_DEMO`, nil)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := row.Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "join" {
		t.Fatalf("got %q", name)
	}
}

func TestManualCommitAndRollback(t *testing.T) {
	d := openDemo(t)
	defer d.Close()
	ctx := context.Background()

	if err := d.Commit(); err == nil {
		t.Fatal("pool Commit must fail")
	}

	tx, err := d.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Begin(ctx); err == nil {
		t.Fatal("Begin inside a transaction must fail")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": "gone"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": "after"}); err == nil {
		t.Fatal("exec after rollback must fail")
	}
	if n := countDemo(t, d); n != 0 {
		t.Fatalf("rollback visible rows=%d", n)
	}

	tx, err = d.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": "kept"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err == nil {
		t.Fatal("second Commit must fail")
	}
	if tx.InTx() {
		t.Fatal("committed session must not stay in a transaction")
	}
	if n := countDemo(t, d); n != 1 {
		t.Fatalf("committed rows=%d", n)
	}
}

func TestTxPanicRollsBackAndPoolStaysUsable(t *testing.T) {
	d := openDemo(t)
	defer d.Close()
	ctx := context.Background()

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic must propagate")
			}
		}()
		_ = d.Tx(ctx, func(tx *db.DB) error {
			if _, err := tx.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": "boom"}); err != nil {
				return err
			}
			panic("dbx-test")
		})
	}()

	if n := countDemo(t, d); n != 0 {
		t.Fatalf("panic left rows=%d", n)
	}
	if _, err := d.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": "after"}); err != nil {
		t.Fatal(err)
	}
	if n := countDemo(t, d); n != 1 {
		t.Fatalf("pool after panic rows=%d", n)
	}
}

func TestTraceTransactionAndExecError(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	var ops []string
	d, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    dsn,
		Trace: func(_ context.Context, e db.Event) {
			ops = append(ops, e.Op)
			if e.Op == "exec" && e.Err != nil && e.Duration < 0 {
				t.Fatalf("duration %s", e.Duration)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE demo (name TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	err = d.Tx(ctx, func(tx *db.DB) error {
		_, err := tx.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": "a"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Exec(ctx, `INSERT INTO missing (name) VALUES (@name)`, db.Args{"name": "x"})
	if err == nil {
		t.Fatal("expected exec error")
	}
	want := []string{"exec", "begin", "exec", "commit", "exec"}
	if stringsJoin(ops) != stringsJoin(want) {
		t.Fatalf("ops %v", ops)
	}
}

func stringsJoin(ss []string) string {
	return fmt.Sprint(ss)
}

func TestConcurrentPoolAndTransactions(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn, MaxOpenConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE demo (name TEXT)`, nil); err != nil {
		t.Fatal(err)
	}

	const n = 8
	var wg sync.WaitGroup
	errCh := make(chan error, n*2)
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_, err := d.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": fmt.Sprintf("auto-%d", i)})
			errCh <- err
		}(i)
		go func(i int) {
			defer wg.Done()
			tx, err := d.Begin(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if _, err := tx.Exec(ctx, `INSERT INTO demo (name) VALUES (@name)`, db.Args{"name": fmt.Sprintf("tx-%d", i)}); err != nil {
				_ = tx.Rollback()
				errCh <- err
				return
			}
			errCh <- tx.Commit()
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := countDemo(t, d); got != n*2 {
		t.Fatalf("rows=%d", got)
	}
}

func openDemo(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "t.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(context.Background(), `CREATE TABLE demo (name TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	return d
}

func countDemo(t *testing.T, d *db.DB) int {
	t.Helper()
	row, err := d.QueryRow(context.Background(), `SELECT COUNT(*) FROM demo`, nil)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := row.Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
