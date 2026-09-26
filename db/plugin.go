package db

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// Plugin 是绑定在一条连接池上的扩展。db 只按下面的可选接口分发，不认识具体插件。
// 新能力实现对应接口即可，不必改会话代码。实例随 Config 进入 Open，并复制到该池开出的事务。
// 不要做全局注册。钩子可能被多个 goroutine 同时调用，实现自己要能并发。
// 钩子里不要再调用同一个 *DB，也不要 panic：调用发生时这条连接可能还没归还。
type Plugin interface {
	// Name 在同一条连接池内必须唯一，且不能为空。
	Name() string
}

// EventPlugin 在驱动调用返回之后收到事件。指标和审计实现本接口。
// 语句日志走 Config.Trace，不占用插件名。QueryRow 成功时驱动推迟到 Scan，因此没有事件。
// 查询的连接要等结果集关闭才归还，这里不要再向同一个 *DB 要连接。
type EventPlugin interface {
	Plugin
	OnEvent(ctx context.Context, e Event)
}

// ContextPlugin 在取连接之前改写本次调用的 context。
// 链路追踪把 span 放进返回值。返回 nil 时保持原 context。不要在这里执行 SQL。
type ContextPlugin interface {
	Plugin
	Before(ctx context.Context, op, query string) context.Context
}

// sessionHooks 是 Open 时从插件列表拆出来的钩子。之后只读，事务会话与池共享这一份指针。
type sessionHooks struct {
	before  []ContextPlugin
	after   []EventPlugin
	closers []io.Closer
	closed  atomic.Bool
}

func buildHooks(plugins []Plugin) (*sessionHooks, error) {
	var h sessionHooks
	seen := make(map[string]struct{}, len(plugins))
	for _, p := range plugins {
		if p == nil {
			continue
		}
		name := p.Name()
		if name == "" {
			return nil, fmt.Errorf("dbx: plugin name is empty")
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("dbx: duplicate plugin %q", name)
		}
		seen[name] = struct{}{}
		matched := false
		if c, ok := p.(ContextPlugin); ok {
			h.before = append(h.before, c)
			matched = true
		}
		if e, ok := p.(EventPlugin); ok {
			h.after = append(h.after, e)
			matched = true
		}
		if c, ok := p.(io.Closer); ok {
			h.closers = append(h.closers, c)
			matched = true
		}
		if !matched {
			return nil, fmt.Errorf("dbx: plugin %q implements no hook", name)
		}
	}
	return &h, nil
}

func (h sessionHooks) active() bool {
	return len(h.before) > 0 || len(h.after) > 0
}

// before 依次改写 ctx。没有 ContextPlugin 时原样返回。
func (d *DB) before(ctx context.Context, op, query string) context.Context {
	if d == nil || d.hooks == nil || len(d.hooks.before) == 0 {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, p := range d.hooks.before {
		if next := p.Before(ctx, op, query); next != nil {
			ctx = next
		}
	}
	return ctx
}

// finish 先调用内置 Trace，再按注册顺序回调 EventPlugin。
// 参数切片由调用方持有，回调不要改它。Trace 先于插件，插件 panic 时日志已经记下。
func (d *DB) finish(ctx context.Context, op, query string, args []any, rows int64, err error, start time.Time) {
	if d == nil {
		return
	}
	after := d.hooks != nil && len(d.hooks.after) > 0
	if d.trace == nil && !after {
		return
	}
	ev := Event{
		Op:    op,
		Query: query,
		Args:  args,
		Err:   err,
		Rows:  rows,
	}
	if !start.IsZero() {
		ev.Duration = time.Since(start)
	}
	if d.trace != nil {
		d.trace(ctx, ev)
	}
	if !after {
		return
	}
	for _, p := range d.hooks.after {
		p.OnEvent(ctx, ev)
	}
}

// mark 在配置了 Trace 或任意钩子时记录开始时间。都没有时返回零值，避免每次取时钟。
func (d *DB) mark() time.Time {
	if d == nil || (d.trace == nil && (d.hooks == nil || !d.hooks.active())) {
		return time.Time{}
	}
	return time.Now()
}

// closePlugins 释放实现了 io.Closer 的插件。连接池已经关闭之后调用。
func (h *sessionHooks) closePlugins() error {
	if h == nil || !h.closed.CompareAndSwap(false, true) {
		return nil
	}
	var err error
	for _, c := range h.closers {
		err = errors.Join(err, c.Close())
	}
	return err
}
