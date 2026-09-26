package clickhouse

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestConvertExtUUIDAndDecimal(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	got, ok, err := convertExt(id, reflect.TypeOf(""))
	if err != nil || !ok || got != id.String() {
		t.Fatalf("uuid: %#v %v %v", got, ok, err)
	}
	same, ok, err := convertExt(id, reflect.TypeOf(uuid.UUID{}))
	if err != nil || ok || same != nil {
		t.Fatalf("same uuid type should pass through: %#v %v %v", same, ok, err)
	}

	dec := decimal.RequireFromString("12.50")
	got, ok, err = convertExt(dec, reflect.TypeOf(""))
	if err != nil || !ok || got != "12.5" && got != "12.50" {
		t.Fatalf("decimal: %#v %v %v", got, ok, err)
	}
	if _, ok, err = convertExt(int64(3), reflect.TypeOf("")); ok || err != nil {
		t.Fatalf("plain int: ok=%v err=%v", ok, err)
	}
}
