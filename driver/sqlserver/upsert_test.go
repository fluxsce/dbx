package sqlserver

import (
	"testing"

	"github.com/fluxsce/dbx/db"
	mssql "github.com/microsoft/go-mssqldb"
)

func TestUpsertSQL(t *testing.T) {
	q := dialect{}.UpsertSQL("t", []string{"id", "name"}, []string{"id"}, []string{"name"}, 1)
	want := "MERGE [t] AS target USING (VALUES (?,?)) AS src ([id],[name]) ON target.[id]=src.[id] WHEN MATCHED THEN UPDATE SET [name]=src.[name] WHEN NOT MATCHED THEN INSERT ([id],[name]) VALUES (src.[id],src.[name]);"
	if q != want {
		t.Fatalf("%s", q)
	}
}

func TestClassify(t *testing.T) {
	d := dialect{}
	if d.Classify(mssql.Error{Number: 2627}) != db.ErrorUnique {
		t.Fatal("unique")
	}
	if d.Classify(mssql.Error{Number: 1205}) != db.ErrorDeadlock {
		t.Fatal("deadlock")
	}
	if d.Classify(&mssql.Error{Number: 1222}) != db.ErrorLock {
		t.Fatal("lock")
	}
	if d.Classify(mssql.Error{Number: 3960}) != db.ErrorSerialization {
		t.Fatal("serialization")
	}
}
