//go:build !oracle

package oracle

import (
	"database/sql"
	"fmt"
)

func open(string) (*sql.DB, error) {
	return nil, fmt.Errorf("dbx: oracle requires -tags oracle and Oracle Instant Client")
}
