package clickhouse

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/sqlite"
)

func TestBulkInsertLeavesTransactionToSQL(t *testing.T) {
	done, err := dialect{}.BulkInsert(context.Background(), nil, "t", []string{"id"}, 2, func(i int) ([]any, error) {
		return []any{i}, nil
	})
	if err != nil || done {
		t.Fatalf("nil session done=%v err=%v", done, err)
	}

	sess, err := db.Open(context.Background(), db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "t.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	tx, err := sess.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	called := false
	done, err = dialect{}.BulkInsert(context.Background(), tx, "t", []string{"id"}, 2, func(i int) ([]any, error) {
		called = true
		return []any{i}, nil
	})
	if err != nil || done || called {
		t.Fatalf("open tx done=%v err=%v called=%v", done, err, called)
	}
}
