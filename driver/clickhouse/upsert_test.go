package clickhouse

import (
	"errors"
	"testing"

	"github.com/fluxsce/dbx/db"
)

func TestNoUpsert(t *testing.T) {
	if _, ok := any(dialect{}).(db.UpsertDialect); ok {
		t.Fatal("clickhouse must not implement upsert")
	}
	d := dialect{}
	if d.Classify(errors.New("code: 159")) != db.ErrorOther {
		t.Fatal("classify")
	}
}
