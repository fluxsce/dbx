package clickhouse

import (
	"context"
	"strings"

	"github.com/fluxsce/dbx/db"
)

// 单个原生批次的行数。发完就提交并归还连接，下一块再取，避免一块占住过多内存。
const nativeBatchRows = 10000

// BulkInsert 在连接池上用驱动的原生批量块。
// 调用方已经处于事务中时不处理，交给标准多行 INSERT，避免盖掉尚未发送的批次。
func (dialect) BulkInsert(ctx context.Context, sess *db.DB, table string, columns []string, n int, row func(i int) ([]any, error)) (bool, error) {
	if sess == nil || sess.InTx() || n < 1 || row == nil {
		return false, nil
	}
	q := "INSERT INTO " + sess.QuoteIdent(table) + " (" + quoteCols(sess, columns) + ")"
	for start := 0; start < n; start += nativeBatchRows {
		if err := ctx.Err(); err != nil {
			return true, err
		}
		end := start + nativeBatchRows
		if end > n {
			end = n
		}
		from := start
		to := end
		err := sess.Tx(ctx, func(tx *db.DB) error {
			return tx.StmtExec(ctx, q, func(exec func(args ...any) error) error {
				for i := from; i < to; i++ {
					vals, err := row(i)
					if err != nil {
						return err
					}
					if err := exec(vals...); err != nil {
						return err
					}
				}
				return nil
			})
		})
		if err != nil {
			return true, err
		}
	}
	return true, nil
}

func quoteCols(sess *db.DB, columns []string) string {
	var b strings.Builder
	for i, name := range columns {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(sess.QuoteIdent(name))
	}
	return b.String()
}
