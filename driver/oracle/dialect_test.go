package oracle

import "testing"

func TestDialect(t *testing.T) {
	d := dialect{}
	if got := d.Rebind("a=? AND b=?"); got != "a=:1 AND b=:2" {
		t.Fatal(got)
	}
	if d.QuoteIdent("id") != `"id"` {
		t.Fatal(d.QuoteIdent("id"))
	}
	if d.LimitSQL() == "" {
		t.Fatal("empty limit")
	}
}
