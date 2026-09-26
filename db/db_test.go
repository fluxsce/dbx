package db

import (
	"context"
	"testing"
	"time"

	"github.com/fluxsce/dbx/record"
	"github.com/fluxsce/dbx/utils"
)

func TestNormalizeDriver(t *testing.T) {
	if NormalizeDriver("PG") != "postgres" {
		t.Fatal("pg alias")
	}
	if NormalizeDriver("sqlite3") != "sqlite" {
		t.Fatal("sqlite3 alias")
	}
}

func TestCompileNamedAndDollar(t *testing.T) {
	d := dollarDialect{}
	q, vals, err := compile(d, "SELECT * FROM t WHERE tenantId = @tenantId AND workspaceId = @workspaceId LIMIT @limit OFFSET @offset", Args{
		"tenantId":    "default",
		"workspaceId": "ws1",
		"limit":       20,
		"offset":      0,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT * FROM t WHERE tenantId = $1 AND workspaceId = $2 LIMIT $3 OFFSET $4"
	if q != want {
		t.Fatalf("got %q", q)
	}
	if len(vals) != 4 || vals[0] != "default" {
		t.Fatalf("vals %v", vals)
	}
}

func TestCompileQMark(t *testing.T) {
	d := qmarkDialect{}
	q, _, err := compile(d, "a = @x AND b = @x", Args{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if q != "a = ? AND b = ?" {
		t.Fatalf("got %q", q)
	}
}

func TestCompileBindsTimeAndFlag(t *testing.T) {
	d := qmarkDialect{}
	_, vals, err := compile(d, "a=@t AND f=@f AND n=@n", Args{
		"t": time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC),
		"f": record.FlagY,
		"n": (*string)(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if vals[0] != "2026-08-21 18:00:00" || vals[1] != "Y" || vals[2] != nil {
		t.Fatalf("%v", vals)
	}
}

func TestCompileIgnoresAtInString(t *testing.T) {
	d := qmarkDialect{}
	q, vals, err := compile(d, `SELECT 'a@b.com' WHERE x=@x -- @skip`, Args{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if q != `SELECT 'a@b.com' WHERE x=? -- @skip` || len(vals) != 1 {
		t.Fatalf("got %q %v", q, vals)
	}
}

func TestQuestionMarkInStringAllowed(t *testing.T) {
	d := qmarkDialect{}
	_, _, err := compile(d, `SELECT '?' WHERE x=@x`, Args{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileInList(t *testing.T) {
	d := dollarDialect{}
	q, vals, err := compile(d, "SELECT id FROM t WHERE id IN (@ids) AND name = @name", Args{
		"ids":  []string{"a", "b"},
		"name": "ada",
	})
	if err != nil {
		t.Fatal(err)
	}
	if q != "SELECT id FROM t WHERE id IN ($1,$2) AND name = $3" {
		t.Fatalf("got %q", q)
	}
	if len(vals) != 3 || vals[0] != "a" || vals[1] != "b" || vals[2] != "ada" {
		t.Fatalf("%v", vals)
	}

	q, vals, err = compile(qmarkDialect{}, "SELECT id FROM t WHERE id NOT IN (@ids)", Args{
		"ids": []int{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q != "SELECT id FROM t WHERE id NOT IN (?)" || vals[0] != 1 {
		t.Fatalf("%q %v", q, vals)
	}

	if _, _, err = compile(qmarkDialect{}, "SELECT id FROM t WHERE id = @ids", Args{"ids": []int{1}}); err == nil {
		t.Fatal("list outside IN")
	}
	if _, _, err = compile(qmarkDialect{}, "SELECT id FROM t WHERE id IN (@ids)", Args{"ids": []int{}}); err == nil {
		t.Fatal("empty list")
	}
	ids := []int{7, 8}
	q, _, err = compile(qmarkDialect{}, "SELECT id FROM t WHERE id IN (\n@ids)", Args{"ids": &ids})
	if err != nil {
		t.Fatal(err)
	}
	if q != "SELECT id FROM t WHERE id IN (\n?,?)" {
		t.Fatalf("got %q", q)
	}
	q, vals, err = compile(qmarkDialect{}, "SELECT id FROM t WHERE blob = @b", Args{"b": []byte{1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if q != "SELECT id FROM t WHERE blob = ?" || len(vals) != 1 {
		t.Fatalf("%q %v", q, vals)
	}
}

func TestExpandNamedSetAndWhereSeparate(t *testing.T) {
	setSQL, setVals, err := expandNamed(`"demoId" = @demoId`, Args{"demoId": "new"})
	if err != nil {
		t.Fatal(err)
	}
	whereSQL, whereVals, err := expandNamed(`"demoId" = @demoId`, Args{"demoId": "old"})
	if err != nil {
		t.Fatal(err)
	}
	if setSQL != `"demoId" = ?` || whereSQL != `"demoId" = ?` {
		t.Fatalf("%q %q", setSQL, whereSQL)
	}
	if setVals[0] != "new" || whereVals[0] != "old" {
		t.Fatalf("%v %v", setVals, whereVals)
	}
	q := utils.RebindDollar("UPDATE t SET " + setSQL + " WHERE " + whereSQL)
	if q != `UPDATE t SET "demoId" = $1 WHERE "demoId" = $2` {
		t.Fatalf("%q", q)
	}
}

func TestPageNormalize(t *testing.T) {
	p := Page{Page: 2, PageSize: 50}
	if p.Offset() != 50 || p.Limit() != 50 {
		t.Fatalf("%+v", p)
	}
	zero := Page{}.Normalize()
	if zero.Page != 1 || zero.PageSize != 20 {
		t.Fatalf("%+v", zero)
	}
	large := Page{Page: 1, PageSize: 500}.Normalize()
	if large.PageSize != 500 {
		t.Fatalf("%+v", large)
	}
}

func TestRejectQuestionMark(t *testing.T) {
	d := qmarkDialect{}
	_, _, err := compile(d, "a = ?", Args{})
	if err == nil {
		t.Fatal("business SQL must not use ?")
	}
}

func TestOpenRequiresDriverAndDSN(t *testing.T) {
	if _, err := Open(context.Background(), Config{DSN: "x"}); err == nil {
		t.Fatal("driver")
	}
	if _, err := Open(context.Background(), Config{Driver: "sqlite"}); err == nil {
		t.Fatal("dsn")
	}
}

func TestTraceCompileError(t *testing.T) {
	var got *Event
	d := &DB{
		dial: qmarkDialect{},
		trace: func(_ context.Context, e Event) {
			got = &e
		},
	}
	_, err := d.Exec(context.Background(), "a = ?", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got == nil || got.Op != "exec" || got.Err == nil {
		t.Fatalf("%+v", got)
	}
}

type qmarkDialect struct{}

func (qmarkDialect) Name() string                  { return "sqlite" }
func (qmarkDialect) Rebind(q string) string        { return q }
func (qmarkDialect) QuoteIdent(name string) string { return name }
func (qmarkDialect) LimitSQL() string              { return "LIMIT @limit OFFSET @offset" }

type dollarDialect struct{}

func (dollarDialect) Name() string                  { return "postgres" }
func (dollarDialect) Rebind(q string) string        { return utils.RebindDollar(q) }
func (dollarDialect) QuoteIdent(name string) string { return name }
func (dollarDialect) LimitSQL() string              { return "LIMIT @limit OFFSET @offset" }

func TestLimitSQLAndFetchCompile(t *testing.T) {
	d := dollarDialect{}
	q, vals, err := compile(d, "SELECT id FROM t ORDER BY id OFFSET @offset ROWS FETCH NEXT @limit ROWS ONLY", Args{"offset": 20, "limit": 10})
	if err != nil {
		t.Fatal(err)
	}
	if q != "SELECT id FROM t ORDER BY id OFFSET $1 ROWS FETCH NEXT $2 ROWS ONLY" {
		t.Fatalf("got %q", q)
	}
	if vals[0] != 20 || vals[1] != 10 {
		t.Fatalf("%v", vals)
	}
}

type wrapDialect struct{ qmarkDialect }

func (wrapDialect) PageSQL(query string) string { return "WRAP " + query }

func TestPageSQLDispatch(t *testing.T) {
	wrapped, err := (&DB{dial: wrapDialect{}}).PageSQL("SELECT id FROM t", Page{})
	if err != nil || wrapped != "WRAP SELECT id FROM t" {
		t.Fatal(wrapped, err)
	}
	plain, err := (&DB{dial: qmarkDialect{}}).PageSQL("SELECT id FROM t ORDER BY id", Page{OrderBy: "name", Desc: true})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT id FROM t ORDER BY name DESC LIMIT @limit OFFSET @offset"
	if plain != want {
		t.Fatal(plain)
	}
	if _, err := (&DB{dial: qmarkDialect{}}).PageSQL("SELECT id FROM t", Page{OrderBy: "name;drop"}); err == nil {
		t.Fatal("order column")
	}
}
