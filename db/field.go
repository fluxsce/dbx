package db

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// colField 描述一个带 db 标签的结构体字段如何映射到列。
type colField struct {
	index     []int  // 含嵌入路径的 FieldByIndex
	column    string // db 标签列名（camelCase）
	pk        bool
	omitEmpty bool
	noUpdate  bool
}

// structSchema 是按类型缓存的列映射，避免每行反射解析标签。
type structSchema struct {
	cols    []colField
	byName  map[string]colField // 精确列名
	byLower map[string]colField // 折叠大小写，兼容 Oracle TENANTID
}

var schemaCache sync.Map // reflect.Type → *structSchema

// loadSchema 按结构体类型取列映射；进程内缓存，类型数有界。
func loadSchema(t reflect.Type) (*structSchema, error) {
	t, err := structType(t)
	if err != nil {
		return nil, err
	}
	if v, ok := schemaCache.Load(t); ok {
		return v.(*structSchema), nil
	}
	cols, err := parseCols(t)
	if err != nil {
		return nil, err
	}
	sch := newSchema(cols)
	actual, _ := schemaCache.LoadOrStore(t, sch)
	return actual.(*structSchema), nil
}

// newSchema 从列清单建精确名与小写名两套索引。
func newSchema(cols []colField) *structSchema {
	sch := &structSchema{
		cols:    cols,
		byName:  make(map[string]colField, len(cols)),
		byLower: make(map[string]colField, len(cols)),
	}
	for _, c := range cols {
		sch.byName[c.column] = c
		sch.byLower[strings.ToLower(c.column)] = c
	}
	return sch
}

// structType 把 *T 解成结构体类型。
func structType(t reflect.Type) (reflect.Type, error) {
	if t == nil {
		return nil, fmt.Errorf("dbx: dest must be struct")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("dbx: dest must be struct, got %s", t.Kind())
	}
	return t, nil
}

// parseCols 读取导出字段的 db 标签；无标签则报错。
func parseCols(t reflect.Type) ([]colField, error) {
	t, err := structType(t)
	if err != nil {
		return nil, err
	}
	var out []colField
	seen := map[string]struct{}{}
	if err := collectCols(t, nil, &out, seen); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("dbx: no db tags on %s", t.Name())
	}
	return out, nil
}

// collectCols 递归展开匿名嵌入结构体，列名按小写去重。
func collectCols(t reflect.Type, prefix []int, out *[]colField, seen map[string]struct{}) error {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		idx := appendIndex(prefix, i)
		tag := f.Tag.Get("db")
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct && (tag == "" || tag == "-") {
				if tag == "-" || f.PkgPath != "" {
					continue // db:"-" 或未导出嵌入不展开
				}
				if err := collectCols(ft, idx, out, seen); err != nil {
					return err
				}
				continue
			}
		}
		if f.PkgPath != "" {
			continue
		}
		if tag == "" || tag == "-" {
			continue
		}
		name, opts := splitTag(tag)
		if name == "" {
			return fmt.Errorf("dbx: field %s: empty db column", f.Name)
		}
		if !validIdent(name) {
			return fmt.Errorf("dbx: invalid column %q", name)
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("dbx: duplicate column %q", name)
		}
		seen[key] = struct{}{}
		*out = append(*out, colField{
			index:     idx,
			column:    name,
			pk:        opts["pk"],
			omitEmpty: opts["omitempty"],
			noUpdate:  opts["noupdate"],
		})
	}
	return nil
}

// appendIndex 复制嵌入路径并追加当前字段下标，避免共享底层数组。
func appendIndex(prefix []int, i int) []int {
	idx := make([]int, len(prefix)+1)
	copy(idx, prefix)
	idx[len(prefix)] = i
	return idx
}

// splitTag 拆 `db:"col,pk,omitempty"` 为列名与选项。
func splitTag(tag string) (name string, opts map[string]bool) {
	opts = map[string]bool{}
	parts := strings.Split(tag, ",")
	name = strings.TrimSpace(parts[0])
	for _, p := range parts[1:] {
		opts[strings.TrimSpace(p)] = true
	}
	return name, opts
}

// validIdent 校验表名/列名：字母或下划线开头，其余为字母数字下划线。
func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func validTable(s string) bool { return validIdent(s) }

// isZero 报告反射值是否为零（含无效值），供 omitempty 与主键非空检查。
func isZero(v reflect.Value) bool {
	return !v.IsValid() || v.IsZero()
}

// validTableErr 在表名非法时返回统一错误。
func validTableErr(table string) error {
	if !validTable(table) {
		return fmt.Errorf("dbx: invalid table name %q", table)
	}
	return nil
}

// structValue 要求 row 为非空 *struct，并返回结构体值与列映射。
func structValue(row any) (reflect.Value, []colField, error) {
	rv := reflect.ValueOf(row)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return reflect.Value{}, nil, fmt.Errorf("dbx: row must be non-nil pointer to struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, nil, fmt.Errorf("dbx: row must be pointer to struct")
	}
	sch, err := loadSchema(rv.Type())
	if err != nil {
		return reflect.Value{}, nil, err
	}
	return rv, sch.cols, nil
}

// walkField 沿嵌入路径取值。alloc 为 true 时给 nil 嵌入指针分配；为 false 时遇到 nil 返回 ok=false。
func walkField(v reflect.Value, index []int, alloc bool) (reflect.Value, bool) {
	for n, i := range index {
		if n > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !alloc || !v.CanSet() {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct || i < 0 || i >= v.NumField() {
			return reflect.Value{}, false
		}
		v = v.Field(i)
	}
	return v, true
}

// lookupColumn 按查询结果列名匹配标签：大小写不敏感，并去掉表前缀与引号。
func lookupColumn(byName, byLower map[string]colField, name string) (colField, bool) {
	name = stripColumnName(name)
	if c, ok := byName[name]; ok {
		return c, true
	}
	if c, ok := byLower[strings.ToLower(name)]; ok {
		return c, true
	}
	return colField{}, false
}

// stripColumnName 去掉引号与 table.col 前缀，只留列名。
func stripColumnName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "`\"[]")
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = strings.Trim(name[i+1:], "`\"[]")
	}
	return name
}
