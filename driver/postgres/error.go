package postgres

import (
	"errors"

	"github.com/fluxsce/dbx/db"
	"github.com/jackc/pgx/v5/pgconn"
)

// Classify 按 SQLSTATE 识别唯一冲突、死锁、锁等待和序列化失败。
func (dialect) Classify(err error) db.ErrorKind {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) {
		return db.ErrorOther
	}
	switch pe.Code {
	case "23505":
		return db.ErrorUnique
	case "40P01":
		return db.ErrorDeadlock
	case "55P03":
		return db.ErrorLock
	case "40001":
		return db.ErrorSerialization
	default:
		return db.ErrorOther
	}
}
