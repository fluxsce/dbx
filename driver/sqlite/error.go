package sqlite

import (
	"errors"
	"strings"

	"github.com/fluxsce/dbx/db"
	sqlite3 "modernc.org/sqlite"
)

// Classify 识别唯一约束，以及 SQLITE_BUSY / SQLITE_LOCKED。
// 外键和非空仍是 ErrorOther。只拿到基本约束码时，再看错误文本里有没有 UNIQUE。
func (dialect) Classify(err error) db.ErrorKind {
	var se *sqlite3.Error
	if !errors.As(err, &se) {
		return db.ErrorOther
	}
	code := se.Code()
	switch code & 0xff {
	case 5, 6: // SQLITE_BUSY, SQLITE_LOCKED
		return db.ErrorLock
	case 19: // SQLITE_CONSTRAINT
		switch code >> 8 {
		case 6, 8, 10: // PRIMARYKEY, UNIQUE, ROWID
			return db.ErrorUnique
		case 0:
			msg := se.Error()
			if strings.Contains(msg, "UNIQUE constraint") || strings.Contains(msg, "PRIMARY KEY constraint") {
				return db.ErrorUnique
			}
			return db.ErrorOther
		default:
			return db.ErrorOther
		}
	default:
		return db.ErrorOther
	}
}
