package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/utils"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUpsertSQL(t *testing.T) {
	q := dialect{}.UpsertSQL("t", []string{"id", "name"}, []string{"id", "tenantId"}, []string{"name"}, 1)
	q = utils.RebindDollar(q)
	want := `INSERT INTO "t" ("id","name") VALUES ($1,$2) ON CONFLICT ("id","tenantId") DO UPDATE SET "name"=EXCLUDED."name"`
	if q != want {
		t.Fatalf("%s", q)
	}
}

func TestClassify(t *testing.T) {
	d := dialect{}
	if d.Classify(&pgconn.PgError{Code: "23505"}) != db.ErrorUnique {
		t.Fatal("unique")
	}
	if d.Classify(&pgconn.PgError{Code: "40P01"}) != db.ErrorDeadlock {
		t.Fatal("deadlock")
	}
	if d.Classify(&pgconn.PgError{Code: "55P03"}) != db.ErrorLock {
		t.Fatal("lock")
	}
	if d.Classify(fmt.Errorf("wrap: %w", &pgconn.PgError{Code: "40001"})) != db.ErrorSerialization {
		t.Fatal("serialization")
	}
	if d.Classify(errors.New("no")) != db.ErrorOther {
		t.Fatal("other")
	}
}
