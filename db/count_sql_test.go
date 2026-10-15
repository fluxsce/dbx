package db_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxsce/dbx/db"
)

func TestCountSQLShapes(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   string
		not    string
		noWide bool
	}{
		{
			name:   "gateway style rewrite",
			in:     `SELECT id, name, COALESCE(x,'') FROM T WHERE a = @a ORDER BY name DESC`,
			want:   `SELECT COUNT(*) AS cnt FROM T WHERE a = @a`,
			not:    `ORDER BY`,
			noWide: true,
		},
		{
			name: "strip limit offset",
			in:   `SELECT id FROM T WHERE a = @a ORDER BY id LIMIT @limit OFFSET @offset`,
			want: `SELECT COUNT(*) AS cnt FROM T WHERE a = @a`,
			not:  `LIMIT`,
		},
		{
			name: "join kept like gateway",
			in:   `SELECT c.id, k.name FROM C c LEFT JOIN K k ON k.id = c.kid WHERE c.t = @t`,
			want: `SELECT COUNT(*) AS cnt FROM C c LEFT JOIN K k ON k.id = c.kid WHERE c.t = @t`,
		},
		{
			name: "outer select over union subquery",
			in: `SELECT source, logId FROM (
  SELECT 'A' AS source, id AS logId FROM A
  UNION ALL
  SELECT 'B', id FROM B
) t WHERE 1=1 ORDER BY logId`,
			want: `SELECT COUNT(*) AS cnt FROM (`,
			not:  `ORDER BY`,
		},
		{
			name: "distinct wraps",
			in:   `SELECT DISTINCT agentId FROM FG_APP_INST WHERE tenantId = @t`,
			want: `SELECT COUNT(*) AS cnt FROM (SELECT DISTINCT agentId FROM FG_APP_INST WHERE tenantId = @t)`,
		},
		{
			name: "group by wraps lean",
			in:   `SELECT agentId, count(*) AS n FROM FG_X GROUP BY agentId`,
			want: `SELECT COUNT(*) AS cnt FROM (SELECT 1 FROM FG_X GROUP BY agentId)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := db.CountSQL(tc.in)
			if !strings.Contains(normalizeSpace(got), normalizeSpace(tc.want)) {
				t.Fatalf("got %q want contain %q", got, tc.want)
			}
			if tc.not != "" && strings.Contains(strings.ToUpper(got), strings.ToUpper(tc.not)) {
				t.Fatalf("got %q should not contain %q", got, tc.not)
			}
			if tc.noWide && strings.Contains(strings.ToLower(got), "coalesce") {
				t.Fatalf("count should not keep wide projection: %s", got)
			}
		})
	}
}

func TestCountSQLExec(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "count.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE T (id TEXT PRIMARY KEY, name TEXT)`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(ctx, `INSERT INTO T VALUES ('a','one'),('b','two'),('c','two')`, nil); err != nil {
		t.Fatal(err)
	}

	n, err := d.Count(ctx, db.CountSQL(`SELECT id, name FROM T WHERE name <> '' ORDER BY id`), nil)
	if err != nil || n != 3 {
		t.Fatalf("simple: %d %v", n, err)
	}
	n, err = d.Count(ctx, db.CountSQL(`SELECT DISTINCT name FROM T`), nil)
	if err != nil || n != 2 {
		t.Fatalf("distinct: %d %v", n, err)
	}
	n, err = d.Count(ctx, db.CountSQL(`SELECT name, count(*) AS c FROM T GROUP BY name`), nil)
	if err != nil || n != 2 {
		t.Fatalf("group: %d %v", n, err)
	}

	var page []struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	total, err := d.SelectPage(ctx, &page, `SELECT id, name FROM T ORDER BY id`, db.Page{Page: 1, PageSize: 1}, nil)
	if err != nil || total != 3 || len(page) != 1 {
		t.Fatalf("SelectPage auto: total=%d page=%+v err=%v", total, page, err)
	}
	total, err = d.SelectPageCount(ctx, &page, "", `SELECT id, name FROM T ORDER BY id`, db.Page{Page: 1, PageSize: 1}, nil)
	if err != nil || total != 3 {
		t.Fatalf("SelectPageCount empty count: total=%d err=%v", total, err)
	}
	total, err = d.SelectPageCount(ctx, &page,
		`SELECT COUNT(*) AS cnt FROM T WHERE name = @name`,
		`SELECT id, name FROM T WHERE name = @name ORDER BY id`,
		db.Page{Page: 1, PageSize: 10},
		db.Args{"name": "two"})
	if err != nil || total != 2 || len(page) != 2 {
		t.Fatalf("SelectPageCount hand count: total=%d page=%+v err=%v", total, page, err)
	}
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
