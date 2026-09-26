package oracle

import (
	"io"
	"reflect"

	"github.com/fluxsce/dbx/db"
)

// godrorPkg 用反射匹配类型，默认构建因此不必导入 godror。
const godrorPkg = "github.com/godror/godror"

// ConvertScan 把 godror.Number 收成数字文本，并读出 LOB。
// CLOB 变成字符串，BLOB 变成 []byte。其它值保持不变。
func (dialect) ConvertScan(src any, _ reflect.Type) (any, bool, error) {
	return convertOracle(src)
}

// ConvertScan 对 Oracle 11g 做同样的 NUMBER 与 LOB 转换。
func (dialect11) ConvertScan(src any, _ reflect.Type) (any, bool, error) {
	return convertOracle(src)
}

// convertOracle 识别 godror 的 Number 与 Lob。nil 指针视为已处理的 NULL。
func convertOracle(src any) (any, bool, error) {
	if src == nil {
		return nil, false, nil
	}
	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, true, nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil, false, nil
	}
	t := rv.Type()
	if t.PkgPath() != godrorPkg {
		return nil, false, nil
	}
	switch t.Name() {
	case "Number":
		return rv.String(), true, nil
	case "Lob":
		return readLob(rv)
	default:
		return nil, false, nil
	}
}

// readLob 按 IsClob 把 LOB 读成字符串或 []byte。
func readLob(rv reflect.Value) (any, bool, error) {
	isClob := false
	if f := rv.FieldByName("IsClob"); f.IsValid() && f.Kind() == reflect.Bool {
		isClob = f.Bool()
	}
	var r io.Reader
	if f := rv.FieldByName("Reader"); f.IsValid() && !f.IsNil() {
		if rd, ok := f.Interface().(io.Reader); ok {
			r = rd
		}
	}
	out, err := readOracleReader(r, isClob)
	return out, true, err
}

// readOracleReader 读完 LOB。reader 为 nil 时，CLOB 得到空串，BLOB 得到 nil 切片。
// 读完后关闭实现了 io.Closer 的 reader，释放 Oracle 定位器。
func readOracleReader(r io.Reader, isClob bool) (out any, err error) {
	if r == nil {
		if isClob {
			return "", nil
		}
		return []byte(nil), nil
	}
	if c, ok := r.(io.Closer); ok {
		defer func() {
			if cerr := c.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if isClob {
		return string(b), nil
	}
	return b, nil
}

var (
	_ db.ScanDialect = dialect{}
	_ db.ScanDialect = dialect11{}
)
