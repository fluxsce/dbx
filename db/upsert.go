package db

import (
	"context"
	"fmt"
	"reflect"
)

// Upsert 在 keys 不存在时插入 row，已存在时更新其余可写列。
// keys 是主键或唯一键的列名，与 db 标签一致，顺序要和该唯一索引一致，至少一列。
// noupdate 的列只出现在插入里，冲突时不改。
// row 与 Insert 相同：*struct、[]struct、[]*struct 或 *[]struct。
// 多行按引擎批量上限拆开；能回滚的引擎在连接池上放进同一事务。
// ClickHouse 不支持本方法。MySQL 与 MariaDB 按实际撞上的唯一键更新，不能在语句里指定键名。
func (d *DB) Upsert(ctx context.Context, table string, row any, keys ...string) error {
	if d == nil || d.pool == nil || d.dial == nil {
		return fmt.Errorf("dbx: nil session")
	}
	if d.done {
		return fmt.Errorf("dbx: transaction already finished")
	}
	if err := validTableErr(table); err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("dbx: upsert: keys are required")
	}
	up, ok := d.dial.(UpsertDialect)
	if !ok {
		return fmt.Errorf("dbx: upsert is not supported on %s", d.dial.Name())
	}
	list, err := asRowList(row)
	if err != nil {
		return err
	}
	sch, err := loadSchema(list[0].Type())
	if err != nil {
		return err
	}
	names, updates, byName, err := upsertPlan(list, sch.cols, keys)
	if err != nil {
		return err
	}
	return d.writeUpsert(ctx, up, table, names, keys, updates, byName, list)
}

// upsertPlan 决定插入列和冲突时要更新的列。键列为零值时返回错误。
func upsertPlan(list []reflect.Value, cols []colField, keys []string) (names, updates []string, byName map[string]colField, err error) {
	byName = make(map[string]colField, len(cols))
	for _, c := range cols {
		byName[c.column] = c
	}
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		c, ok := byName[key]
		if !ok {
			return nil, nil, nil, fmt.Errorf("dbx: upsert: unknown key %q", key)
		}
		if _, ok := seen[key]; ok {
			return nil, nil, nil, fmt.Errorf("dbx: upsert: duplicate key %q", key)
		}
		seen[key] = struct{}{}
		for _, rv := range list {
			fv, ok := walkField(rv, c.index, false)
			if !ok || isZero(fv) {
				return nil, nil, nil, fmt.Errorf("dbx: upsert: key %s is empty", key)
			}
		}
	}
	keySet := seen
	names, err = insertColumns(list[0], cols)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("dbx: upsert: %w", err)
	}
	for _, key := range keys {
		found := false
		for _, name := range names {
			if name == key {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, nil, fmt.Errorf("dbx: upsert: key %s is empty", key)
		}
	}
	for _, name := range names {
		if _, isKey := keySet[name]; isKey || byName[name].noUpdate {
			continue
		}
		updates = append(updates, name)
	}
	if len(updates) == 0 {
		return nil, nil, nil, fmt.Errorf("dbx: upsert: no columns to update")
	}
	return names, updates, byName, nil
}

// writeUpsert 按 InsertLimit 分批执行方言给出的语句。
func (d *DB) writeUpsert(ctx context.Context, up UpsertDialect, table string, names, keys, updates []string, byName map[string]colField, list []reflect.Value) error {
	lim := normalizedInsertLimit(d.dial)
	size := chunkRows(lim, len(names))
	if len(list) > size && lim.Atomic && !d.InTx() {
		return d.Tx(ctx, func(tx *DB) error {
			return tx.execUpsertChunks(ctx, up, table, names, keys, updates, byName, list, size)
		})
	}
	return d.execUpsertChunks(ctx, up, table, names, keys, updates, byName, list, size)
}

func (d *DB) execUpsertChunks(ctx context.Context, up UpsertDialect, table string, names, keys, updates []string, byName map[string]colField, list []reflect.Value, size int) error {
	if size < 1 {
		size = 1
	}
	for start := 0; start < len(list); start += size {
		end := start + size
		if end > len(list) {
			end = len(list)
		}
		part := list[start:end]
		query := up.UpsertSQL(table, names, keys, updates, len(part))
		vals, err := bindRows(nil, part, names, byName)
		if err != nil {
			return err
		}
		if _, err := d.execBound(ctx, query, vals); err != nil {
			return err
		}
	}
	return nil
}
