package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/sqlite"
)

func TestSelectInList(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id   int
		name string
	}{{1, "ada"}, {2, "grace"}, {3, "lin"}} {
		if _, err := d.Exec(ctx, `INSERT INTO t (id, name) VALUES (@id, @name)`, db.Args{"id": row.id, "name": row.name}); err != nil {
			t.Fatal(err)
		}
	}
	type item struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	var got []item
	err = d.Select(ctx, &got, `SELECT id, name FROM t WHERE id IN (@ids) AND name <> @skip ORDER BY id`, db.Args{
		"ids":  []int{1, 3},
		"skip": "grace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "ada" || got[1].Name != "lin" {
		t.Fatalf("%+v", got)
	}
}
