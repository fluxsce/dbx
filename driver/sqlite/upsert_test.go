package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/sqlite"
)

type upsertRow struct {
	ID      string `db:"id"`
	Name    string `db:"name"`
	Created string `db:"created,noupdate"`
}

func TestSQLiteUpsertAndUnique(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY, name TEXT, created TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	row := &upsertRow{ID: "a", Name: "ada", Created: "c1"}
	if err := d.Upsert(ctx, "t", row, "id"); err != nil {
		t.Fatal(err)
	}
	row.Name = "grace"
	row.Created = "c2"
	if err := d.Upsert(ctx, "t", row, "id"); err != nil {
		t.Fatal(err)
	}
	got := upsertRow{}
	if err := d.Get(ctx, &got, `SELECT id, name, created FROM t WHERE id=@id`, db.Args{"id": "a"}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "grace" || got.Created != "c1" {
		t.Fatalf("%+v", got)
	}
	if _, err := d.Exec(ctx, `INSERT INTO t (id, name, created) VALUES (@id, @name, @created)`, db.Args{
		"id": "a", "name": "x", "created": "c3",
	}); err == nil {
		t.Fatal("expected unique violation")
	} else if kind := d.Classify(err); kind != db.ErrorUnique || kind.Retryable() {
		t.Fatalf("kind %s retry %v err %v", kind, kind.Retryable(), err)
	}
	if err := d.Upsert(ctx, "t", &upsertRow{}, "id"); err == nil {
		t.Fatal("empty key")
	}
}
