package oracle

import (
	"errors"
	"testing"

	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/utils"
)

func TestUpsertSQL(t *testing.T) {
	d := dialect{}
	q := utils.RebindColon(d.UpsertSQL("t", []string{"id", "name"}, []string{"id"}, []string{"name"}, 2))
	want := `MERGE INTO "t" target USING (SELECT :1 AS "id",:2 AS "name" FROM dual UNION ALL SELECT :3,:4 FROM dual) src ON (target."id"=src."id") WHEN MATCHED THEN UPDATE SET "name"=src."name" WHEN NOT MATCHED THEN INSERT ("id","name") VALUES (src."id",src."name")`
	if q != want {
		t.Fatalf("%s", q)
	}
	d11 := dialect11{}
	if d11.UpsertSQL("t", []string{"id", "name"}, []string{"id"}, []string{"name"}, 1) != d.UpsertSQL("t", []string{"id", "name"}, []string{"id"}, []string{"name"}, 1) {
		t.Fatal("11g merge")
	}
}

func TestClassify(t *testing.T) {
	d := dialect{}
	d11 := dialect11{}
	if d.Classify(errors.New("ORA-00001: unique constraint")) != db.ErrorUnique {
		t.Fatal("unique")
	}
	if d.Classify(errors.New("ORA-00060: deadlock")) != db.ErrorDeadlock {
		t.Fatal("deadlock")
	}
	if d11.Classify(errors.New("ORA-00054: resource busy")) != db.ErrorLock {
		t.Fatal("lock")
	}
	if d.Classify(errors.New("ORA-08177: can't serialize access")) != db.ErrorSerialization {
		t.Fatal("serialization")
	}
	if d.Classify(errors.New("ORA-00942: table or view does not exist")) != db.ErrorOther {
		t.Fatal("other")
	}
}
