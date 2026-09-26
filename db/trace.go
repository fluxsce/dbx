package db

import (
	"context"
	"time"
)

// Event 是一次已经结束的驱动调用。
// Query 是发给驱动的 SQL。Args 是按顺序绑定的值。
// Rows 在执行语句上是 RowsAffected；查询在调用方读完结果前不知道行数，因此保持 0。
// Duration 是驱动返回前经过的时间。
type Event struct {
	Op       string
	Query    string
	Args     []any
	Duration time.Duration
	Err      error
	Rows     int64
}

// TraceFunc 是内置语句日志，在每条语句以及 Begin、Commit、Rollback 之后收到 Event。
// nil 表示不记录。它不是插件，不占用 Plugins 里的名字。
// 回调不得 panic，也不要再调用同一个 *DB。Commit 与 Rollback 传入 context.Background。
type TraceFunc func(ctx context.Context, e Event)
