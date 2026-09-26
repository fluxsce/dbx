// Package livetest 跑 PostgreSQL 与 SQL Server 共用的真库场景。
// 调用方先空白导入对应驱动，再传入该引擎的列类型。
package livetest

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/record"
)

type row struct {
	ID     string         `db:"id,pk"`
	Name   string         `db:"demoName"`
	Amount record.Decimal `db:"amount"`
	Flag   record.Flag    `db:"on_flag"`
	At     *time.Time     `db:"at"`
	Yes    bool           `db:"yes"`
}

// Run 覆盖命名参数、大小写列名、空值、事务回滚、分页、Upsert 和唯一冲突。
func Run(t *testing.T, driver, dsn, table, amountType, timeType, boolType string) {
	t.Helper()
	var events []db.Event
	d, err := db.Open(context.Background(), db.Config{
		Driver: driver,
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
	qt := d.QuoteIdent(table)
	if _, err := d.Exec(ctx, "DROP TABLE IF EXISTS "+qt, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = d.Exec(context.Background(), "DROP TABLE IF EXISTS "+qt, nil)
	})
	if _, err := d.Exec(ctx, createSQL(d, qt, amountType, timeType, boolType), nil); err != nil {
		t.Fatal(err)
	}

	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ada := row{ID: "u-ada", Name: "ada", Amount: "12.50", Flag: record.FlagY, At: &when, Yes: true}
	if err := d.Insert(ctx, table, &ada); err != nil {
		t.Fatal(err)
	}
	if err := d.Insert(ctx, table, &row{ID: "u-empty", Name: "empty-time", Flag: "", Yes: false}); err != nil {
		t.Fatal(err)
	}

	cols, err := d.QuoteColumns(&row{})
	if err != nil {
		t.Fatal(err)
	}
	q := `SELECT ` + cols + ` FROM ` + qt + ` WHERE ` + d.QuoteIdent("id") + `=@id`
	var got row
	if err := d.Get(ctx, &got, q, db.Args{"id": ada.ID}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "ada" || got.Flag != record.FlagY || !got.Yes || got.At == nil {
		t.Fatalf("%+v", got)
	}
	if got.Amount != "12.50" && got.Amount != "12.5" {
		t.Fatalf("amount %q", got.Amount)
	}
	if got.At.Format("2006-01-02 15:04:05") != "2026-01-02 03:04:05" {
		t.Fatalf("at %s", got.At)
	}
	var empty row
	if err := d.Get(ctx, &empty, q, db.Args{"id": "u-empty"}); err != nil {
		t.Fatal(err)
	}
	if empty.At != nil || empty.Flag != record.FlagN || empty.Yes || empty.Amount != "" {
		t.Fatalf("nulls %+v", empty)
	}

	err = d.Tx(ctx, func(tx *db.DB) error {
		return tx.Insert(ctx, table, &row{ID: "u-kept", Name: "kept", Amount: "3.00", Flag: record.FlagY, Yes: true})
	})
	if err != nil {
		t.Fatal(err)
	}
	err = d.Tx(ctx, func(tx *db.DB) error {
		if err := tx.Insert(ctx, table, &row{ID: "u-rolled", Name: "rolled", Amount: "1.00", Flag: record.FlagN, Yes: false}); err != nil {
			return err
		}
		return errForce
	})
	if err == nil {
		t.Fatal("expected rollback")
	}
	if err := d.Get(ctx, &got, q, db.Args{"id": "u-rolled"}); err != sql.ErrNoRows {
		t.Fatalf("rolled back row: %v", err)
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic must propagate")
			}
		}()
		_ = d.Tx(ctx, func(tx *db.DB) error {
			if err := tx.Insert(ctx, table, &row{ID: "u-boom", Name: "boom", Amount: "1.00", Flag: record.FlagN, Yes: false}); err != nil {
				return err
			}
			panic("dbx-test")
		})
	}()
	if err := d.Get(ctx, &got, q, db.Args{"id": "u-boom"}); err != sql.ErrNoRows {
		t.Fatalf("panic row: %v", err)
	}
	if err := d.Insert(ctx, table, &row{ID: "u-after", Name: "after", Amount: "2.00", Flag: record.FlagY, Yes: true}); err != nil {
		t.Fatal(err)
	}

	var page []row
	total, err := d.SelectPage(ctx, &page,
		`SELECT COUNT(*) FROM `+qt,
		`SELECT `+cols+` FROM `+qt+` ORDER BY `+d.QuoteIdent("demoName"),
		db.Page{Page: 1, PageSize: 1, OrderBy: "demoName"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(page) != 1 || page[0].Name != "ada" {
		t.Fatalf("page total=%d rows=%v", total, page)
	}

	var picked []row
	err = d.Select(ctx, &picked, `SELECT `+cols+` FROM `+qt+` WHERE `+d.QuoteIdent("id")+` IN (@ids)`, db.Args{
		"ids": []string{ada.ID, "missing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(picked) != 1 || picked[0].ID != ada.ID {
		t.Fatalf("in %+v", picked)
	}

	ada.Name = "ada lovelace"
	if err := d.Upsert(ctx, table, &ada, "id"); err != nil {
		t.Fatal(err)
	}
	if err := d.Get(ctx, &got, q, db.Args{"id": ada.ID}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "ada lovelace" {
		t.Fatalf("upsert %q", got.Name)
	}
	err = d.Insert(ctx, table, &row{ID: ada.ID, Name: "dup", Flag: record.FlagY, Yes: true})
	if err == nil {
		t.Fatal("expected unique violation")
	}
	if gotKind := d.Classify(err); gotKind != db.ErrorUnique || gotKind.Retryable() {
		t.Fatalf("classify %s: %v", gotKind, err)
	}

	var sawBegin, sawCommit, sawRollback bool
	for _, e := range events {
		switch e.Op {
		case "begin":
			sawBegin = true
		case "commit":
			sawCommit = true
		case "rollback":
			sawRollback = true
		}
	}
	if !sawBegin || !sawCommit || !sawRollback {
		t.Fatalf("trace begin=%v commit=%v rollback=%v", sawBegin, sawCommit, sawRollback)
	}
}

func createSQL(d *db.DB, table, amountType, timeType, boolType string) string {
	q := d.QuoteIdent
	return `CREATE TABLE ` + table + ` (
		` + q("id") + ` VARCHAR(64) NOT NULL PRIMARY KEY,
		` + q("demoName") + ` VARCHAR(64) NOT NULL,
		` + q("amount") + ` ` + amountType + ` NULL,
		` + q("on_flag") + ` CHAR(1) NOT NULL,
		` + q("at") + ` ` + timeType + ` NULL,
		` + q("yes") + ` ` + boolType + ` NOT NULL
	)`
}

var errForce = errString("force rollback")

type errString string

func (e errString) Error() string { return string(e) }
