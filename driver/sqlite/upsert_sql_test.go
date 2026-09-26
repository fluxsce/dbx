package sqlite

import "testing"

func TestUpsertSQL(t *testing.T) {
	q := dialect{}.UpsertSQL("t", []string{"id", "name", "created"}, []string{"id"}, []string{"name"}, 2)
	want := `INSERT INTO "t" ("id","name","created") VALUES (?,?,?),(?,?,?) ON CONFLICT ("id") DO UPDATE SET "name"=excluded."name"`
	if q != want {
		t.Fatalf("%s", q)
	}
}
