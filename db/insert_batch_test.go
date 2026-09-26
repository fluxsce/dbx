package db

import (
	"strings"
	"testing"
)

type limitDialect struct {
	lim InsertLimit
}

func (limitDialect) Name() string                  { return "fake" }
func (limitDialect) Rebind(q string) string        { return q }
func (limitDialect) QuoteIdent(name string) string { return `"` + name + `"` }
func (limitDialect) LimitSQL() string              { return "" }
func (d limitDialect) InsertLimit() InsertLimit    { return d.lim }

func TestInsertChunkRespectsParams(t *testing.T) {
	// 10 列、2000 个参数：单批最多 200 行，而不是声明的 1000 行。
	lim := normalizedInsertLimit(limitDialect{lim: InsertLimit{MaxRows: 1000, MaxParams: 2000, Atomic: true, Prepare: true}})
	rows := chunkRows(lim, 10)
	if rows != 200 || !lim.Atomic || !lim.Prepare {
		t.Fatalf("rows=%d %+v", rows, lim)
	}
	lim = normalizedInsertLimit(limitDialect{lim: InsertLimit{MaxRows: 500, MaxParams: 32766, Atomic: true, Prepare: true}})
	rows = chunkRows(lim, 6)
	if rows != 500 || !lim.Atomic {
		t.Fatalf("sqlite-like rows=%d %+v", rows, lim)
	}
	lim = normalizedInsertLimit(limitDialect{lim: InsertLimit{MaxRows: 500, MaxParams: 2000, Atomic: true}})
	if rows = chunkRows(lim, 3000); rows != 1 {
		t.Fatalf("wide row should still insert one at a time, got %d", rows)
	}
	lim = normalizedInsertLimit(limitDialect{lim: InsertLimit{MaxRows: 10, MaxParams: 100, Atomic: false, Prepare: false}})
	if lim.Atomic || lim.Prepare {
		t.Fatalf("%+v", lim)
	}
	// 未实现 InsertDialect 时用默认，并允许预编译。
	lim = normalizedInsertLimit(nil)
	if lim.MaxRows != defaultInsertRows || lim.MaxParams != defaultInsertParams || !lim.Atomic || !lim.Prepare {
		t.Fatalf("default %+v", lim)
	}
}

func TestInsertValuesSQLShape(t *testing.T) {
	q := insertValuesSQL(limitDialect{}, "AI_DEMO", []string{"id", "name"}, 2)
	if q != `INSERT INTO "AI_DEMO" ("id","name") VALUES (?,?),(?,?)` {
		t.Fatal(q)
	}
	if strings.Count(q, "?") != 4 {
		t.Fatal(q)
	}
}
