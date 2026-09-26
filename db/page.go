package db

const (
	defaultPage     = 1  // 页码从 1 起
	defaultPageSize = 20 // 未传 pageSize 时的默认条数
)

// Page 是分页参数。页码从 1 起，PageSize 是每页条数。
// OrderBy 为排序列，空则沿用 SQL 里已有的 ORDER BY。Desc 为 true 时降序。
// PageSize 小于 1 时用 20。这里不设上限，调用方在进入查询前自行收束。
type Page struct {
	Page     int
	PageSize int
	OrderBy  string
	Desc     bool
}

// Normalize 把小于 1 的页码收成 1，把小于 1 的 pageSize 收成 20。
func (p Page) Normalize() Page {
	if p.Page < 1 {
		p.Page = defaultPage
	}
	if p.PageSize < 1 {
		p.PageSize = defaultPageSize
	}
	return p
}

// Offset 返回 LIMIT/OFFSET 的偏移（(page-1)*pageSize）。
func (p Page) Offset() int {
	p = p.Normalize()
	return (p.Page - 1) * p.PageSize
}

// Limit 返回每页条数。
func (p Page) Limit() int {
	return p.Normalize().PageSize
}

// BindPage 复制 args 后写入 limit / offset，供方言分页子句使用。
// 不修改调用方传入的 map。键名与方言无关。
func BindPage(args Args, page Page) Args {
	out := cloneArgs(args)
	page = page.Normalize()
	out["limit"] = page.Limit()
	out["offset"] = page.Offset()
	return out
}

// LimitSQL 返回当前引擎的分页子句。具体文本在各自方言包里。
func (d *DB) LimitSQL() string {
	if d == nil || d.dial == nil {
		return ""
	}
	return d.dial.LimitSQL()
}

// PageSQL 按当前引擎给 query 分页，并在 OrderBy 非空时替换最外层排序。
// 后缀分页由方言的 LimitSQL 提供。Oracle 11g 等实现 PageDialect，改写整句。
func (d *DB) PageSQL(query string, page Page) (string, error) {
	q, err := d.applyOrder(query, page)
	if err != nil {
		return "", err
	}
	if d == nil || d.dial == nil {
		return q, nil
	}
	if p, ok := d.dial.(PageDialect); ok {
		return p.PageSQL(q), nil // 不能追加后缀的引擎改写整句，例如 Oracle 11g
	}
	if clause := d.dial.LimitSQL(); clause != "" {
		return q + " " + clause, nil // 子句文本来自当前方言，这里只拼接
	}
	return q, nil
}
