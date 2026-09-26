# 从 sqlx 迁入

[English](from-sqlx.en.md)

dbx 覆盖 sqlx 的同一类工作：命名 SQL、结构体扫描、`IN` 列表、占位符改写和事务。单表 `Insert`、`Update`、`Delete`、`Upsert`、分页和 `Classify` 是额外能力。各引擎形态见 [引擎](engines.zh-CN.md)。

## 占位符

sqlx 按绑定类型接受 `?`、`$1` 或 `:name`。dbx 只接受 `@name`。键不带 `@`。

```go
// sqlx
db.NamedExec(`INSERT INTO users (id, name) VALUES (:id, :name)`,
    map[string]any{"id": "u1", "name": "ada"})

// dbx
_, err = d.Exec(ctx, `INSERT INTO users (id, name) VALUES (@id, @name)`,
    db.Args{"id": "u1", "name": "ada"})
```

`@@` 写成字面量 `@`。业务 SQL 中的 `?` 是错误，包括原先靠 `Rebind` 处理的语句。

## 会话

sqlx 使用 `*sqlx.DB` 和 `*sqlx.Tx`。dbx 两者都是 `*db.DB`。同一个值上的方法相同。

```go
err = d.Tx(ctx, func(tx *db.DB) error {
    _, err := tx.Exec(ctx, `UPDATE users SET name = @name WHERE id = @id`,
        db.Args{"id": "u1", "name": "ada lovelace"})
    return err
})
```

事务里再 `Begin` 会返回错误。事务里再 `Tx` 会加入当前事务。没有保存点 API。连接池可以共用。事务值只留在一个 goroutine 上。

## 扫描

`Get` 和 `Select` 按 `db` 标签匹配列。每个列都要有标签。没有标签的导出字段不是列。

```go
type User struct {
    ID   string `db:"id,pk"`
    Name string `db:"name"`
}
```

扫描目标是结构体：`*struct`、`*[]struct`、`*[]*struct`。`map[string]any` 和 `[]string` 不能作为扫描目标。`Each` 把行流进同一个结构体。回调若要留下该值，先拷贝。`Select` 覆盖切片。

## `IN`

```go
err = d.Select(ctx, &users, `SELECT id, name FROM users WHERE id IN (@ids)`,
    db.Args{"ids": []string{"u1", "u2"}})
```

空切片是错误。列表可能为空时，另外写一条语句。`[]byte` 仍是一个参数。

## 写入

```go
func save(ctx context.Context, d *db.DB, user *User) error {
    if err := d.Insert(ctx, "users", user); err != nil {
        return err
    }
    cond, err := d.PK(user)
    if err != nil {
        return err
    }
    user.Name = "ada lovelace"
    return d.Update(ctx, "users", user, cond)
}
```

`PK` 读取带 `,pk` 的字段。`Where` 的片段使用 `@name`，不写 `WHERE` 关键字。空条件是错误。

`Upsert(ctx, table, row, keyColumns...)` 用一条语句完成插入或更新。键列顺序与唯一索引一致。ClickHouse 返回错误。MySQL 与 MariaDB 按实际撞上的唯一键更新。

## 错误

`Classify` 把驱动错误收成 `unique`、`deadlock`、`lock`、`serialization` 或 `other`。`Retryable()` 为真时重试整段事务。唯一冲突在数据改变之前一直是失败。返回的错误仍是驱动错误。
