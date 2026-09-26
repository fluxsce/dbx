//go:build oracle

package oracle

import (
	"database/sql"

	_ "github.com/godror/godror"
)

func open(dsn string) (*sql.DB, error) {
	return sql.Open("godror", dsn)
}
