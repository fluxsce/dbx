package oracle

import (
	"strings"
	"testing"
)

func TestOracle11PageSQL(t *testing.T) {
	d := dialect11{}
	got := d.PageSQL("SELECT id FROM t WHERE name = @name ORDER BY id")
	if !strings.Contains(got, "ROW_NUMBER() OVER (ORDER BY id)") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "WHERE name = @name") || strings.Count(got, "ORDER BY id") != 1 {
		t.Fatal(got)
	}
	if !strings.Contains(got, "dbx_rn > @offset AND dbx_rn <= @offset + @limit") {
		t.Fatal(got)
	}
	if d.Rebind("a=?") != "a=:1" {
		t.Fatal(d.Rebind("a=?"))
	}
}

func TestOracle11KeepsInnerOrderBy(t *testing.T) {
	got := dialect11{}.PageSQL("SELECT * FROM (SELECT id FROM t ORDER BY id) s ORDER BY name")
	if !strings.Contains(got, "SELECT id FROM t ORDER BY id") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "OVER (ORDER BY name)") {
		t.Fatal(got)
	}
}

func TestOracle11DefaultOrder(t *testing.T) {
	got := dialect11{}.PageSQL("SELECT id FROM t WHERE note = 'order by x'")
	if !strings.Contains(got, "OVER (ORDER BY ROWID)") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "'order by x'") {
		t.Fatal(got)
	}
}
