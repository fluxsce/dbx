package sqlserver

import (
	"errors"

	"github.com/fluxsce/dbx/db"
	mssql "github.com/microsoft/go-mssqldb"
)

// Classify 按 SQL Server 错误号识别唯一冲突、死锁、锁等待和快照冲突。
func (dialect) Classify(err error) db.ErrorKind {
	var me mssql.Error
	if errors.As(err, &me) {
		return sqlServerKind(me.Number)
	}
	var mp *mssql.Error
	if errors.As(err, &mp) && mp != nil {
		return sqlServerKind(mp.Number)
	}
	return db.ErrorOther
}

func sqlServerKind(number int32) db.ErrorKind {
	switch number {
	case 2601, 2627:
		return db.ErrorUnique
	case 1205:
		return db.ErrorDeadlock
	case 1222:
		return db.ErrorLock
	case 3960:
		return db.ErrorSerialization
	default:
		return db.ErrorOther
	}
}
