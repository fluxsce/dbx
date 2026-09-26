package clickhouse

import (
	"fmt"
	"reflect"

	"github.com/fluxsce/dbx/db"
	"github.com/fluxsce/dbx/record"
)

// ConvertScan 把 ClickHouse 扩展值收成公共扫描能接受的文本。
// 包括实现了 fmt.Stringer 的 UUID、Decimal、Int128、Int256 和 Geo。
// Array 与 Map 仍是 Go 切片和映射，由公共赋值按元素转换。
// 字段类型本身已是该扩展类型时不转换。
func (dialect) ConvertScan(src any, dst reflect.Type) (any, bool, error) {
	return convertExt(src, dst)
}

// convertExt 只处理 ClickHouse 扩展包里的值。nil 指针视为已处理的 NULL。
func convertExt(src any, dst reflect.Type) (any, bool, error) {
	if src == nil || dst == nil {
		return nil, false, nil
	}
	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, true, nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || !rv.CanInterface() {
		return nil, false, nil
	}
	t := rv.Type()
	if !extPkg(t.PkgPath()) {
		return nil, false, nil
	}
	if t.AssignableTo(baseType(dst)) {
		return nil, false, nil
	}
	if !textDest(dst) {
		return nil, false, nil
	}
	if s, ok := rv.Interface().(fmt.Stringer); ok {
		return s.String(), true, nil
	}
	if baseType(dst).Kind() == reflect.String {
		return fmt.Sprint(rv.Interface()), true, nil
	}
	return nil, false, nil
}

// extPkg 报告类型是否来自 ClickHouse 的扩展包。
func extPkg(pkg string) bool {
	switch pkg {
	case "github.com/google/uuid",
		"github.com/shopspring/decimal",
		"github.com/ClickHouse/ch-go/proto",
		"github.com/paulmach/orb":
		return true
	default:
		return false
	}
}

// baseType 剥掉字段类型上的指针，得到元素类型。
func baseType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// textDest 报告字段是否接受文本：字符串、数字、record.Decimal 或 record.Flag。
func textDest(dst reflect.Type) bool {
	t := baseType(dst)
	if t == nil {
		return false
	}
	if t == reflect.TypeOf(record.Decimal("")) || t == reflect.TypeOf(record.Flag("")) {
		return true
	}
	switch t.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

var _ db.ScanDialect = dialect{}
