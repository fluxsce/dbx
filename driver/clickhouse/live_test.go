package clickhouse_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/clickhouse"
	"github.com/fluxsce/dbx/record"
)

// 真库场景覆盖 ClickHouse 与 SQLite / MySQL 不同的部分：
// MergeTree 建表、跳数索引、UUID / Decimal / Array 扫描、
// ALTER UPDATE / DELETE、分页，以及驱动并不撤销语句的事务。
// 设置 DBX_CLICKHOUSE_DSN 才运行。建议带上 mutations_sync=1，缺了会自动补上。
// 例：clickhouse://user:pass@127.0.0.1:9000/db?dial_timeout=10s&mutations_sync=1
// 未设置时跳过，默认 go test 不连接这台机器。
func TestLiveClickHouse(t *testing.T) {
	dsn := os.Getenv("DBX_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("DBX_CLICKHOUSE_DSN is empty")
	}
	if !strings.Contains(dsn, "mutations_sync=") {
		if strings.Contains(dsn, "?") {
			dsn += "&mutations_sync=1"
		} else {
			dsn += "?mutations_sync=1"
		}
	}

	var events []db.Event
	d, err := db.Open(context.Background(), db.Config{
		Driver: "clickhouse",
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
	table := "dbx_it_ch"
	if _, err := d.Exec(ctx, "DROP TABLE IF EXISTS "+table, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Exec(context.Background(), "DROP TABLE IF EXISTS "+table, nil)
	})
	ddl := `CREATE TABLE ` + table + ` (
		id UInt64,
		name String,
		amount Nullable(Decimal(12, 2)),
		on_flag String,
		at Nullable(DateTime),
		yes Bool,
		uid UUID,
		tags Array(String),
		INDEX idx_name name TYPE bloom_filter GRANULARITY 1
	) ENGINE = MergeTree ORDER BY id`
	if _, err := d.Exec(ctx, ddl, nil); err != nil {
		t.Fatal(err)
	}

	createSQL := showCreate(t, d, table)
	if !strings.Contains(createSQL, "idx_name") || !strings.Contains(createSQL, "bloom_filter") || !strings.Contains(createSQL, "ORDER BY") {
		t.Fatalf("create sql missing index or sort key: %s", createSQL)
	}

	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ada := chRow{
		ID: 1, Name: "ada", Amount: "12.50", Flag: record.FlagY, At: &when, Yes: true,
		UID: "11111111-1111-1111-1111-111111111111", Tags: []string{"red", "blue"},
	}
	if err := d.Insert(ctx, table, &ada); err != nil {
		t.Fatal(err)
	}
	empty := chRow{
		ID: 2, Name: "empty-time", Amount: "", Flag: "", Yes: false,
		UID: "22222222-2222-2222-2222-222222222222", Tags: []string{},
	}
	if err := d.Insert(ctx, table, &empty); err != nil {
		t.Fatal(err)
	}
	batch := []chRow{
		{ID: 3, Name: "bob", Amount: "1.00", Flag: record.FlagY, Yes: true, UID: "33333333-3333-3333-3333-333333333333", Tags: []string{"g"}},
		{ID: 4, Name: "cara", Amount: "2.00", Flag: record.FlagN, Yes: false, UID: "44444444-4444-4444-4444-444444444444", Tags: []string{"h"}},
	}
	if err := d.Insert(ctx, table, batch); err != nil {
		t.Fatal(err)
	}

	q := `SELECT id, name, amount, on_flag, at, yes, uid, tags FROM ` + table + ` WHERE id=@id`
	var got chRow
	if err := d.Get(ctx, &got, q, db.Args{"id": uint64(1)}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "ada" || !sameAmount(got.Amount, "12.50") || got.Flag != record.FlagY || !got.Yes || got.At == nil {
		t.Fatalf("%+v", got)
	}
	if got.At.Format("2006-01-02 15:04:05") != "2026-01-02 03:04:05" {
		t.Fatalf("at %s", got.At)
	}
	if !strings.EqualFold(got.UID, ada.UID) || len(got.Tags) != 2 || got.Tags[0] != "red" || got.Tags[1] != "blue" {
		t.Fatalf("uuid/tags %+v", got)
	}

	var byName chRow
	if err := d.Get(ctx, &byName, `SELECT id, name FROM `+table+` WHERE name=@name`, db.Args{"name": "ada"}); err != nil {
		t.Fatal(err)
	}
	if byName.ID != 1 {
		t.Fatalf("index lookup %+v", byName)
	}

	var blank chRow
	if err := d.Get(ctx, &blank, q, db.Args{"id": uint64(2)}); err != nil {
		t.Fatal(err)
	}
	if blank.At != nil || blank.Flag != record.FlagN || blank.Yes || blank.Amount != "" {
		t.Fatalf("nulls %+v", blank)
	}

	var seen int
	var item chRow
	if err := d.Each(ctx, &item, `SELECT id, name FROM `+table+` WHERE id IN (1, 3) ORDER BY id`, nil, func() error {
		seen++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if seen != 2 {
		t.Fatalf("each %d", seen)
	}

	if err := d.Update(ctx, table, &chAmount{Amount: "99.00"}, db.Where("id = @id", db.Args{"id": uint64(1)})); err != nil {
		t.Fatal(err)
	}
	waitRow(t, d, q, 1, func(r chRow) bool { return sameAmount(r.Amount, "99") && r.Name == "ada" })

	if err := d.Delete(ctx, table, db.Where("id = @id", db.Args{"id": uint64(2)})); err != nil {
		t.Fatal(err)
	}
	waitGone(t, d, q, 2)

	err = d.Tx(ctx, func(tx *db.DB) error {
		return tx.Insert(ctx, table, &chRow{
			ID: 5, Name: "kept", Amount: "3.00", Flag: record.FlagY, Yes: true,
			UID: "55555555-5555-5555-5555-555555555555", Tags: []string{},
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Get(ctx, &got, q, db.Args{"id": uint64(5)}); err != nil {
		t.Fatal(err)
	}

	// clickhouse-go 的 database/sql 事务不会缓存语句。回滚只丢掉连接，已执行的 INSERT 仍在。
	err = d.Tx(ctx, func(tx *db.DB) error {
		if err := tx.Insert(ctx, table, &chRow{
			ID: 6, Name: "rolled", Amount: "1.00", Flag: record.FlagN, Yes: false,
			UID: "66666666-6666-6666-6666-666666666666", Tags: []string{},
		}); err != nil {
			return err
		}
		return errForce
	})
	if err == nil {
		t.Fatal("expected rollback")
	}
	if err := d.Get(ctx, &got, q, db.Args{"id": uint64(6)}); err != nil {
		t.Fatalf("rolled row must stay visible on clickhouse: %v", err)
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic must propagate")
			}
		}()
		_ = d.Tx(ctx, func(tx *db.DB) error {
			if err := tx.Insert(ctx, table, &chRow{
				ID: 7, Name: "boom", Amount: "1.00", Flag: record.FlagN, Yes: false,
				UID: "77777777-7777-7777-7777-777777777777", Tags: []string{},
			}); err != nil {
				return err
			}
			panic("dbx-test")
		})
	}()
	if err := d.Get(ctx, &got, q, db.Args{"id": uint64(7)}); err != nil {
		t.Fatalf("panic row must stay visible on clickhouse: %v", err)
	}
	if err := d.Insert(ctx, table, &chRow{
		ID: 8, Name: "after", Amount: "2.00", Flag: record.FlagY, Yes: true,
		UID: "88888888-8888-8888-8888-888888888888", Tags: []string{},
	}); err != nil {
		t.Fatal(err)
	}

	var page []chRow
	total, err := d.SelectPage(ctx, &page,
		`SELECT toInt64(count()) FROM `+table,
		`SELECT id, name, amount, on_flag, at, yes, uid, tags FROM `+table+` ORDER BY name`,
		db.Page{Page: 1, PageSize: 1, OrderBy: "name"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 7 || len(page) != 1 || page[0].Name != "ada" {
		t.Fatalf("page total=%d rows=%v", total, page)
	}

	if _, err := d.Exec(ctx, "SELECT * FROM dbx_it_missing", nil); err == nil {
		t.Fatal("expected exec error")
	}

	var sawBegin, sawCommit, sawRollback, sawExecErr bool
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
		case "exec":
			if e.Err != nil {
				sawExecErr = true
			}
		}
	}
	if !sawBegin || !sawCommit || !sawRollback || !sawExecErr {
		t.Fatalf("trace begin=%v commit=%v rollback=%v execErr=%v", sawBegin, sawCommit, sawRollback, sawExecErr)
	}
}

type chRow struct {
	ID     uint64         `db:"id,pk,noupdate"`
	Name   string         `db:"name"`
	Amount record.Decimal `db:"amount"`
	Flag   record.Flag    `db:"on_flag"`
	At     *time.Time     `db:"at"`
	Yes    bool           `db:"yes"`
	UID    string         `db:"uid"`
	Tags   []string       `db:"tags"`
}

type chAmount struct {
	Amount record.Decimal `db:"amount"`
}

func showCreate(t *testing.T, d *db.DB, table string) string {
	t.Helper()
	rows, err := d.Query(context.Background(), "SHOW CREATE TABLE "+table, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, v := range raw {
		b.WriteString(asText(v))
		b.WriteByte('\n')
	}
	return b.String()
}

func asText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return ""
	}
}

func sameAmount(got record.Decimal, want string) bool {
	return trimAmount(string(got)) == trimAmount(want)
}

func trimAmount(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

func waitRow(t *testing.T, d *db.DB, query string, id uint64, ok func(chRow) bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var last chRow
	var lastErr error
	for time.Now().Before(deadline) {
		last = chRow{}
		lastErr = d.Get(context.Background(), &last, query, db.Args{"id": id})
		if lastErr == nil && ok(last) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("row %d not updated: %+v err=%v", id, last, lastErr)
}

func waitGone(t *testing.T, d *db.DB, query string, id uint64) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		var row chRow
		last = d.Get(context.Background(), &row, query, db.Args{"id": id})
		if last == sql.ErrNoRows {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("row %d still visible: %v", id, last)
}

var errForce = errString("force rollback")

type errString string

func (e errString) Error() string { return string(e) }
