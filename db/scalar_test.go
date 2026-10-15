package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fluxsce/dbx/db"
)

func TestCountExistsAndSelectPage(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scalar.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE T (id TEXT NOT NULL PRIMARY KEY, name TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(ctx, `INSERT INTO T (id, name) VALUES ('a', 'one'), ('b', 'two')`, nil); err != nil {
		t.Fatal(err)
	}

	n, err := d.Count(ctx, `SELECT count(*) FROM T`, nil)
	if err != nil || n != 2 {
		t.Fatalf("count all: %d %v", n, err)
	}
	n, err = d.Count(ctx, `SELECT count(*) FROM T WHERE id = @id`, db.Args{"id": "z"})
	if err != nil || n != 0 {
		t.Fatalf("count zero: %d %v", n, err)
	}

	ok, err := d.Exists(ctx, `SELECT 1 FROM T WHERE id = @id LIMIT 1`, db.Args{"id": "a"})
	if err != nil || !ok {
		t.Fatalf("exists a: %v %v", ok, err)
	}
	ok, err = d.Exists(ctx, `SELECT 1 FROM T WHERE id = @id LIMIT 1`, db.Args{"id": "z"})
	if err != nil || ok {
		t.Fatalf("exists z: %v %v", ok, err)
	}

	var page []struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	total, err := d.SelectPage(ctx, &page,
		`SELECT id, name FROM T ORDER BY id`,
		db.Page{Page: 1, PageSize: 1}, nil)
	if err != nil || total != 2 || len(page) != 1 || page[0].ID != "a" {
		t.Fatalf("SelectPage auto count: total=%d page=%+v err=%v", total, page, err)
	}

	total, err = d.SelectPageCount(ctx, &page,
		`SELECT count(*) FROM T WHERE id = @id`,
		`SELECT id, name FROM T WHERE id = @id ORDER BY id`,
		db.Page{Page: 1, PageSize: 10},
		db.Args{"id": "b"})
	if err != nil || total != 1 || len(page) != 1 || page[0].ID != "b" {
		t.Fatalf("SelectPageCount: total=%d page=%+v err=%v", total, page, err)
	}
}
