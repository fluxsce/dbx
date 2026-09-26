package utils

import "testing"

func TestRebind(t *testing.T) {
	if got := RebindDollar("a=? AND b=?"); got != "a=$1 AND b=$2" {
		t.Fatal(got)
	}
	if got := RebindColon("a=? AND b=?"); got != "a=:1 AND b=:2" {
		t.Fatal(got)
	}
}

func TestQuote(t *testing.T) {
	if got := Quote(`na"me`, `"`); got != `"na""me"` {
		t.Fatal(got)
	}
	if got := Quote("id", "`"); got != "`id`" {
		t.Fatal(got)
	}
	if got := QuoteBracket("a]b"); got != "[a]]b]" {
		t.Fatal(got)
	}
}
