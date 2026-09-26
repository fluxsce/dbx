package clickhouse

import "github.com/fluxsce/dbx/db"

// Classify 始终返回 ErrorOther。
// MergeTree 收下重复键，不在写入当时报唯一冲突，也没有可回滚事务上的死锁。
func (dialect) Classify(error) db.ErrorKind { return db.ErrorOther }
