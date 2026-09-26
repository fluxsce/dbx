package sqlserver

import "testing"

func TestDialect(t *testing.T) {
	d := dialect{}
	if d.Name() != "sqlserver" {
		t.Fatal(d.Name())
	}
	if got := d.Rebind("a=? AND b=?"); got != "a=? AND b=?" {
		t.Fatal(got)
	}
	if d.QuoteIdent("id") != "[id]" || d.QuoteIdent("a]b") != "[a]]b]" {
		t.Fatal(d.QuoteIdent("a]b"))
	}
	if d.LimitSQL() != "OFFSET @offset ROWS FETCH NEXT @limit ROWS ONLY" {
		t.Fatal(d.LimitSQL())
	}
}
