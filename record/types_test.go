package record

import (
	"testing"
	"time"
)

func TestParseFlag(t *testing.T) {
	y, err := ParseFlag("y")
	if err != nil || !y.Bool() {
		t.Fatalf("Y: %v %v", y, err)
	}
	n, err := ParseFlag("")
	if err != nil || n != FlagN {
		t.Fatalf("empty: %v %v", n, err)
	}
	if _, err := ParseFlag("yes"); err == nil {
		t.Fatal("want error")
	}
	if FlagFromBool(true) != FlagY || FlagFromBool(false) != FlagN {
		t.Fatal("from bool")
	}
}

func TestParseDateTime(t *testing.T) {
	tm, err := ParseDateTime("2026-08-21 15:04:05")
	if err != nil {
		t.Fatal(err)
	}
	if FormatDateTime(tm) != "2026-08-21 15:04:05" {
		t.Fatalf("got %s", FormatDateTime(tm))
	}
	loc := time.FixedZone("CST", 8*3600)
	wall := time.Date(2026, 8, 21, 18, 0, 0, 0, loc)
	if FormatDateTime(wall) != "2026-08-21 18:00:00" {
		t.Fatalf("wall clock got %s", FormatDateTime(wall))
	}
	if s := FormatDateTime(time.Time{}); s != "" {
		t.Fatalf("zero got %q", s)
	}
}

func TestParseDecimal(t *testing.T) {
	d := ParseDecimal(" 12.50 ")
	if d.String() != "12.50" || d.Empty() {
		t.Fatalf("%q", d)
	}
}
