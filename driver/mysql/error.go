package mysql

import (
	"errors"

	"github.com/fluxsce/dbx/db"
	mysqldriver "github.com/go-sql-driver/mysql"
)

// Classify 识别 MySQL / MariaDB 的唯一冲突、死锁和锁等待。
func (dialect) Classify(err error) db.ErrorKind {
	var me *mysqldriver.MySQLError
	if !errors.As(err, &me) {
		return db.ErrorOther
	}
	switch me.Number {
	case 1062, 1169, 1586:
		return db.ErrorUnique
	case 1213:
		return db.ErrorDeadlock
	case 1205:
		return db.ErrorLock
	default:
		return db.ErrorOther
	}
}
