package db

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/fluxsce/dbx/record"
)

func TestAssignNullAndPointer(t *testing.T) {
	var p *string
	dv := reflect.ValueOf(&p).Elem()
	if err := assignField(dv, nil); err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Fatal("NULL must stay nil pointer")
	}

	if err := assignField(dv, "hi"); err != nil {
		t.Fatal(err)
	}
	if p == nil || *p != "hi" {
		t.Fatalf("got %#v", p)
	}

	var s string
	sv := reflect.ValueOf(&s).Elem()
	if err := assignField(sv, nil); err != nil {
		t.Fatal(err)
	}
	if s != "" {
		t.Fatalf("NULL string got %q", s)
	}

	var ns sql.NullString
	nv := reflect.ValueOf(&ns).Elem()
	if err := assignField(nv, nil); err != nil {
		t.Fatal(err)
	}
	if ns.Valid {
		t.Fatal("NullString NULL")
	}
	if err := assignField(nv, "x"); err != nil {
		t.Fatal(err)
	}
	if !ns.Valid || ns.String != "x" {
		t.Fatalf("%+v", ns)
	}
}

func TestAssignBoolAndPointer(t *testing.T) {
	var b bool
	if err := assignField(reflect.ValueOf(&b).Elem(), int64(1)); err != nil {
		t.Fatal(err)
	}
	if !b {
		t.Fatal("int64 1")
	}
	var p *bool
	if err := assignField(reflect.ValueOf(&p).Elem(), "false"); err != nil {
		t.Fatal(err)
	}
	if p == nil || *p {
		t.Fatalf("%#v", p)
	}
	if err := assignField(reflect.ValueOf(&p).Elem(), nil); err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Fatal("NULL bool pointer")
	}
}

func TestAssignNilBytesAsNull(t *testing.T) {
	var p *string
	dv := reflect.ValueOf(&p).Elem()
	var b []byte
	if err := assignField(dv, b); err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Fatal("nil []byte is NULL")
	}
	if err := assignField(dv, []byte("ab")); err != nil {
		t.Fatal(err)
	}
	if p == nil || *p != "ab" {
		t.Fatalf("%#v", p)
	}
}

func TestBindPointerTimeFlag(t *testing.T) {
	var sp *string
	v, err := bindValue(reflect.ValueOf(sp))
	if err != nil || v != nil {
		t.Fatalf("nil *string: %v %v", v, err)
	}
	s := ""
	sp = &s
	v, err = bindValue(reflect.ValueOf(sp))
	if err != nil || v != "" {
		t.Fatalf("empty *string: %v %v", v, err)
	}

	v, err = bindValue(reflect.ValueOf(time.Time{}))
	if err != nil || v != nil {
		t.Fatalf("zero time: %v %v", v, err)
	}
	tm := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	v, err = bindValue(reflect.ValueOf(tm))
	if err != nil || v != "2026-01-02 03:04:05" {
		t.Fatalf("time: %v %v", v, err)
	}

	v, err = bindValue(reflect.ValueOf(record.Flag("")))
	if err != nil || v != "N" {
		t.Fatalf("empty flag: %v %v", v, err)
	}
	v, err = bindValue(reflect.ValueOf(record.Decimal("")))
	if err != nil || v != nil {
		t.Fatalf("empty decimal: %v %v", v, err)
	}

	ns := sql.NullString{String: "a", Valid: true}
	v, err = bindValue(reflect.ValueOf(ns))
	if err != nil || v != "a" {
		t.Fatalf("NullString: %v %v", v, err)
	}
	v, err = bindValue(reflect.ValueOf(sql.NullString{}))
	if err != nil || v != nil {
		t.Fatalf("invalid NullString: %v %v", v, err)
	}

	nt := sql.NullTime{Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Valid: true}
	v, err = bindValue(reflect.ValueOf(nt))
	if err != nil || v != "2026-01-02 03:04:05" {
		t.Fatalf("NullTime: %v %v", v, err)
	}
}

func TestAssignIntFromText(t *testing.T) {
	var n int
	dv := reflect.ValueOf(&n).Elem()
	if err := assignField(dv, "42"); err != nil || n != 42 {
		t.Fatalf("%d %v", n, err)
	}
	var dec record.Decimal
	dd := reflect.ValueOf(&dec).Elem()
	if err := assignField(dd, int64(19)); err != nil || dec.String() != "19" {
		t.Fatalf("%q %v", dec, err)
	}
}

func TestAssignUnsignedAndSlice(t *testing.T) {
	var n int64
	if err := assignField(reflect.ValueOf(&n).Elem(), uint64(7)); err != nil || n != 7 {
		t.Fatalf("uint64: %d %v", n, err)
	}
	if err := assignField(reflect.ValueOf(&n).Elem(), uint64(1<<63)); err == nil {
		t.Fatal("uint64 overflow must fail")
	}

	var nums []int64
	if err := assignField(reflect.ValueOf(&nums).Elem(), []uint32{1, 2}); err != nil || len(nums) != 2 || nums[1] != 2 {
		t.Fatalf("slice: %#v %v", nums, err)
	}
	var dict map[string]int
	if err := assignField(reflect.ValueOf(&dict).Elem(), map[string]uint8{"a": 3}); err != nil || dict["a"] != 3 {
		t.Fatalf("map: %#v %v", dict, err)
	}

	var s string
	tm := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := assignField(reflect.ValueOf(&s).Elem(), tm); err != nil || s != "2026-01-02 03:04:05" {
		t.Fatalf("time text: %q %v", s, err)
	}
	if err := assignField(reflect.ValueOf(&s).Elem(), textID("abc")); err != nil || s != "abc" {
		t.Fatalf("stringer: %q %v", s, err)
	}

	var p *int
	src := 9
	if err := assignField(reflect.ValueOf(&p).Elem(), &src); err != nil || p == nil || *p != 9 {
		t.Fatalf("deref: %#v %v", p, err)
	}
}

type textID string

func (t textID) String() string { return string(t) }
