package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/sqlite"
)

type ctxKey struct{}

type orderPlugin struct {
	name    string
	events  *[]string
	closed  *int
	sawSpan bool
}

func (p *orderPlugin) Name() string { return p.name }

func (p *orderPlugin) Before(ctx context.Context, _, _ string) context.Context {
	return context.WithValue(ctx, ctxKey{}, p.name)
}

func (p *orderPlugin) OnEvent(ctx context.Context, e db.Event) {
	if ctx.Value(ctxKey{}) == p.name {
		p.sawSpan = true
	}
	*p.events = append(*p.events, p.name+":"+e.Op)
}

func (p *orderPlugin) Close() error {
	*p.closed++
	return nil
}

func TestPluginsRunInOrderAndCloseWithPool(t *testing.T) {
	var events []string
	var closed int
	first := &orderPlugin{name: "audit", events: &events, closed: &closed}
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    dsn,
		Plugins: []db.Plugin{
			first,
			recPlugin{name: "trace", events: &events},
		},
		Trace: func(_ context.Context, e db.Event) {
			events = append(events, "builtin:"+e.Op)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(context.Background(), `CREATE TABLE t (id INTEGER)`, nil); err != nil {
		t.Fatal(err)
	}
	if !first.sawSpan {
		t.Fatal("context plugin value did not reach the event")
	}
	want := []string{"builtin:exec", "audit:exec", "trace:exec"}
	if len(events) != len(want) {
		t.Fatalf("events %v", events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events %v", events)
		}
	}

	tx, err := d.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	if closed != 0 {
		t.Fatalf("tx close released plugin: %d", closed)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("pool close count %d", closed)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("second close count %d", closed)
	}
}

func TestPluginConfigRejectedBeforeOpen(t *testing.T) {
	_, err := db.Open(context.Background(), db.Config{
		Driver:  "sqlite",
		DSN:     filepath.Join(t.TempDir(), "t.db"),
		Plugins: []db.Plugin{nil, eventPlugin("log"), eventPlugin("log")},
	})
	if err == nil || err.Error() != `dbx: duplicate plugin "log"` {
		t.Fatalf("duplicate: %v", err)
	}

	_, err = db.Open(context.Background(), db.Config{
		Driver:  "sqlite",
		DSN:     filepath.Join(t.TempDir(), "t.db"),
		Plugins: []db.Plugin{namedPlugin("")},
	})
	if err == nil || err.Error() != "dbx: plugin name is empty" {
		t.Fatalf("empty name: %v", err)
	}

	_, err = db.Open(context.Background(), db.Config{
		Driver:  "sqlite",
		DSN:     filepath.Join(t.TempDir(), "t.db"),
		Plugins: []db.Plugin{namedPlugin("orphan")},
	})
	if err == nil || err.Error() != `dbx: plugin "orphan" implements no hook` {
		t.Fatalf("no hook: %v", err)
	}
}

type namedPlugin string

func (n namedPlugin) Name() string { return string(n) }

type eventPlugin string

func (e eventPlugin) Name() string { return string(e) }

func (eventPlugin) OnEvent(context.Context, db.Event) {}

type recPlugin struct {
	name   string
	events *[]string
}

func (p recPlugin) Name() string { return p.name }

func (p recPlugin) OnEvent(_ context.Context, e db.Event) {
	*p.events = append(*p.events, p.name+":"+e.Op)
}
