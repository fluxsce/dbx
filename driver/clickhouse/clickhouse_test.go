package clickhouse

import "testing"

func TestMutationSQL(t *testing.T) {
	d := dialect{}
	if got := d.UpdateSQL("`t`", "name = ?", "id = ?"); got != "ALTER TABLE `t` UPDATE name = ? WHERE id = ?" {
		t.Fatal(got)
	}
	if got := d.DeleteSQL("`t`", "id = ?"); got != "ALTER TABLE `t` DELETE WHERE id = ?" {
		t.Fatal(got)
	}
	if d.LimitSQL() == "" || d.QuoteIdent("id") != "`id`" {
		t.Fatal(d.LimitSQL(), d.QuoteIdent("id"))
	}
	lim := d.InsertLimit()
	if lim.MaxRows != 1000 || lim.MaxParams != 100000 || lim.Atomic || lim.Prepare {
		t.Fatalf("insert limit %+v", lim)
	}
}
