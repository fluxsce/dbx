package oracle

import (
	"strings"

	"github.com/fluxsce/dbx/db"
)

// Classify 从错误文本里的 ORA- 号识别分类。默认构建不导入 godror。
func (dialect) Classify(err error) db.ErrorKind { return classifyOracle(err) }

// Classify 与 12c 使用同一组 ORA- 号。
func (dialect11) Classify(err error) db.ErrorKind { return classifyOracle(err) }

func classifyOracle(err error) db.ErrorKind {
	if err == nil {
		return db.ErrorOther
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "ORA-00001"):
		return db.ErrorUnique
	case strings.Contains(msg, "ORA-00060"):
		return db.ErrorDeadlock
	case strings.Contains(msg, "ORA-00054"), strings.Contains(msg, "ORA-30006"):
		return db.ErrorLock
	case strings.Contains(msg, "ORA-08177"):
		return db.ErrorSerialization
	default:
		return db.ErrorOther
	}
}
