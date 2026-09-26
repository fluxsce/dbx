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

// TraceFunc 在每条语句以及 Begin、Commit、Rollback 之后收到 Event。
// nil 表示不记录。回调不得 panic。
// Commit 与 Rollback 没有上下文参数，因此传入 context.Background。
type TraceFunc func(ctx context.Context, e Event)

// finish 在 trace 非空时回调一次 Event。
func (d *DB) finish(ctx context.Context, op, query string, args []any, rows int64, err error, start time.Time) {
	if d == nil || d.trace == nil {
		return
	}
	d.trace(ctx, Event{
		Op:       op,
		Query:    query,
		Args:     args,
		Duration: time.Since(start),
		Err:      err,
		Rows:     rows,
	})
}

// mark 在需要记录耗时时返回开始时间。未配置 Trace 时返回零值，避免每次取时钟。
func (d *DB) mark() time.Time {
	if d == nil || d.trace == nil {
		return time.Time{}
	}
	return time.Now()
}
