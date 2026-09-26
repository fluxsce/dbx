package mysql

import (
	"testing"

	"github.com/fluxsce/dbx/db"
	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestUpsertSQL(t *testing.T) {
	q := dialect{}.UpsertSQL("t", []string{"id", "name"}, []string{"id"}, []string{"name"}, 2)
	want := "INSERT INTO `t` (`id`,`name`) VALUES (?,?),(?,?) ON DUPLICATE KEY UPDATE `name`=VALUES(`name`)"
	if q != want {
		t.Fatalf("%s", q)
	}
}

func TestClassify(t *testing.T) {
	d := dialect{}
	if d.Classify(&mysqldriver.MySQLError{Number: 1062}) != db.ErrorUnique {
		t.Fatal("unique")
	}
	if d.Classify(&mysqldriver.MySQLError{Number: 1213}) != db.ErrorDeadlock {
		t.Fatal("deadlock")
	}
	if d.Classify(&mysqldriver.MySQLError{Number: 1205}) != db.ErrorLock {
		t.Fatal("lock")
	}
	if d.Classify(&mysqldriver.MySQLError{Number: 1205}).Retryable() != true {
		t.Fatal("retry")
	}
	if d.Classify(&mysqldriver.MySQLError{Number: 1146}) != db.ErrorOther {
		t.Fatal("other")
	}
}
