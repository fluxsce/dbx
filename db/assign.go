package db

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/fluxsce/dbx/record"
)

// assignField 把驱动扫出的值写入结构体字段：NULL→零值，指针按需分配，支持 Scanner / Flag / Decimal / time。
// 切片、映射和整数宽度在这里按元素转换。驱动专用类型先由 ScanDialect 收成这些值。
func assignField(dst reflect.Value, src any) error {
	if !dst.CanSet() {
		return fmt.Errorf("cannot set field")
	}
	var null bool
	src, null = derefSrc(src)
	if !null {
		src = unwrapBytes(src)
		null = isSQLNull(src)
	}
	if null {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	if dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem())) // 非 NULL 时分配 *T
		}
		return assignField(dst.Elem(), src)
	}
	if dst.CanAddr() {
		if sc, ok := dst.Addr().Interface().(sql.Scanner); ok {
			return sc.Scan(src)
		}
	}
	switch dst.Type() {
	case reflect.TypeOf(record.Flag("")):
		s, err := asString(src)
		if err != nil {
			return err
		}
		f, err := record.ParseFlag(s)
		if err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(f))
		return nil
	case reflect.TypeOf(record.Decimal("")):
		s, err := asString(src)
		if err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(record.ParseDecimal(s)))
		return nil
	case reflect.TypeOf(time.Time{}):
		t, err := asTime(src)
		if err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(t))
		return nil
	}
	sv := reflect.ValueOf(src)
	if !sv.IsValid() {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	if isNumberKind(dst.Kind()) {
		// 先按宽度检查，避免 uint64 转 int64 时被截成负数
		if err := assignNumber(dst, sv); err == nil {
			return nil
		} else if err != errSkipPrimitive {
			return err
		}
	}
	if dst.Kind() == reflect.Slice && sv.Kind() == reflect.Slice && !sv.Type().AssignableTo(dst.Type()) {
		// 元素类型不同才逐个转，例如 []uint32 写入 []int64；[]byte 仍当文本
		if err := assignSlice(dst, sv); err == nil {
			return nil
		} else if err != errSkipPrimitive {
			return err
		}
	}
	if dst.Kind() == reflect.Map && sv.Kind() == reflect.Map && !sv.Type().AssignableTo(dst.Type()) {
		if err := assignMap(dst, sv); err == nil {
			return nil
		} else if err != errSkipPrimitive {
			return err
		}
	}
	if sv.Type().AssignableTo(dst.Type()) {
		dst.Set(sv)
		return nil
	}
	if sv.Type().ConvertibleTo(dst.Type()) {
		cv := sv.Convert(dst.Type())
		dst.Set(cv)
		return nil
	}
	if dst.Kind() == reflect.Bool {
		b, err := asBool(src)
		if err != nil {
			return err
		}
		dst.SetBool(b)
		return nil
	}
	if err := assignPrimitive(dst, src); err == nil {
		return nil
	} else if err != errSkipPrimitive {
		return err
	}
	if dst.Kind() == reflect.String {
		s, err := asString(src)
		if err != nil {
			return err
		}
		dst.SetString(s)
		return nil
	}
	return fmt.Errorf("cannot assign %T to %s", src, dst.Type())
}

// asBool 接受 bool、0/1 整数，以及 true/false/0/1 文本。指针字段由 assignField 先解引用。
func asBool(src any) (bool, error) {
	switch v := src.(type) {
	case bool:
		return v, nil
	case int:
		return v != 0, nil
	case int8:
		return v != 0, nil
	case int16:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case uint:
		return v != 0, nil
	case uint8:
		return v != 0, nil
	case uint16:
		return v != 0, nil
	case uint32:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "t":
			return true, nil
		case "0", "false", "f":
			return false, nil
		default:
			return false, fmt.Errorf("cannot assign %q to bool", v)
		}
	default:
		return false, fmt.Errorf("cannot assign %T to bool", src)
	}
}

// errSkipPrimitive 表示源值不能按数字解析，交给后续字符串赋值。
var errSkipPrimitive = fmt.Errorf("skip primitive")

// assignPrimitive 把文本/数字扫进 int、uint、float；解析失败返回 errSkipPrimitive。
func assignPrimitive(dst reflect.Value, src any) error {
	s, err := asString(src)
	if err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return errSkipPrimitive
	}
	switch dst.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return errSkipPrimitive
		}
		if dst.OverflowInt(n) {
			return fmt.Errorf("overflow %s: %s", dst.Type(), s)
		}
		dst.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return errSkipPrimitive
		}
		if dst.OverflowUint(n) {
			return fmt.Errorf("overflow %s: %s", dst.Type(), s)
		}
		dst.SetUint(n)
		return nil
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return errSkipPrimitive
		}
		if dst.OverflowFloat(n) {
			return fmt.Errorf("overflow %s: %s", dst.Type(), s)
		}
		dst.SetFloat(n)
		return nil
	default:
		return errSkipPrimitive
	}
}

// isSQLNull 报告驱动值是否表示 SQL NULL（含 nil 的 []byte / RawBytes）。
func isSQLNull(src any) bool {
	if src == nil {
		return true
	}
	switch v := src.(type) {
	case []byte:
		return v == nil
	case sql.RawBytes:
		return v == nil
	default:
		return false
	}
}

// unwrapBytes 把 []byte / RawBytes 转成 string；nil 切片保持为 NULL。
func unwrapBytes(src any) any {
	switch v := src.(type) {
	case []byte:
		if v == nil {
			return nil
		}
		return string(v)
	case sql.RawBytes:
		if v == nil {
			return nil
		}
		return string(v)
	default:
		return src
	}
}

// asString 将驱动值转为文本；数字用十进制，不用 fmt.Sprint 以免科学计数法。
func asString(src any) (string, error) {
	src = unwrapBytes(src)
	if isSQLNull(src) {
		return "", nil
	}
	switch v := src.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case time.Time:
		return record.FormatDateTime(v), nil
	case *time.Time:
		if v == nil {
			return "", nil
		}
		return record.FormatDateTime(*v), nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprint(src), nil
	}
}

// derefSrc 解开驱动返回的一层或多层指针。nil 指针视为 SQL NULL。
func derefSrc(src any) (any, bool) {
	if src == nil {
		return nil, true
	}
	rv := reflect.ValueOf(src)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, true
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || !rv.CanInterface() {
		return src, false
	}
	return rv.Interface(), false
}

// isNumberKind 报告字段是否为整数或浮点。布尔和字符串不在此列。
func isNumberKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// assignNumber 按目标宽度写入整数和浮点。无符号到有符号超出范围时报错，不截断成负数。
func assignNumber(dst reflect.Value, src reflect.Value) error {
	switch src.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setFromInt(dst, src.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setFromUint(dst, src.Uint())
	case reflect.Float32, reflect.Float64:
		return setFromFloat(dst, src.Float())
	default:
		return errSkipPrimitive
	}
}

// setFromInt 把有符号整数写入目标宽度。超出范围或写入无符号时为负则返回错误。
func setFromInt(dst reflect.Value, n int64) error {
	switch dst.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if dst.OverflowInt(n) {
			return fmt.Errorf("overflow %s: %d", dst.Type(), n)
		}
		dst.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n < 0 {
			return fmt.Errorf("cannot assign negative %d to %s", n, dst.Type())
		}
		if dst.OverflowUint(uint64(n)) {
			return fmt.Errorf("overflow %s: %d", dst.Type(), n)
		}
		dst.SetUint(uint64(n))
		return nil
	case reflect.Float32, reflect.Float64:
		f := float64(n)
		if dst.OverflowFloat(f) {
			return fmt.Errorf("overflow %s: %d", dst.Type(), n)
		}
		dst.SetFloat(f)
		return nil
	default:
		return errSkipPrimitive
	}
}

// setFromUint 把无符号整数写入目标宽度。大于有符号上限时返回错误，不截成负数。
func setFromUint(dst reflect.Value, n uint64) error {
	switch dst.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n > uint64(math.MaxInt64) || dst.OverflowInt(int64(n)) {
			return fmt.Errorf("overflow %s: %d", dst.Type(), n)
		}
		dst.SetInt(int64(n))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if dst.OverflowUint(n) {
			return fmt.Errorf("overflow %s: %d", dst.Type(), n)
		}
		dst.SetUint(n)
		return nil
	case reflect.Float32, reflect.Float64:
		f := float64(n)
		if dst.OverflowFloat(f) {
			return fmt.Errorf("overflow %s: %d", dst.Type(), n)
		}
		dst.SetFloat(f)
		return nil
	default:
		return errSkipPrimitive
	}
}

// setFromFloat 把浮点写入浮点字段，或截断小数后写入整数。NaN、Inf 和超出范围返回错误。
func setFromFloat(dst reflect.Value, n float64) error {
	switch dst.Kind() {
	case reflect.Float32, reflect.Float64:
		if math.IsNaN(n) || math.IsInf(n, 0) || dst.OverflowFloat(n) {
			return fmt.Errorf("cannot assign %v to %s", n, dst.Type())
		}
		dst.SetFloat(n)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("cannot assign %v to %s", n, dst.Type())
		}
		trunc := math.Trunc(n)
		if dst.Kind() >= reflect.Uint && dst.Kind() <= reflect.Uint64 {
			if trunc < 0 || trunc > float64(math.MaxUint64) {
				return fmt.Errorf("overflow %s: %v", dst.Type(), n)
			}
			return setFromUint(dst, uint64(trunc))
		}
		if trunc < float64(math.MinInt64) || trunc > float64(math.MaxInt64) {
			return fmt.Errorf("overflow %s: %v", dst.Type(), n)
		}
		return setFromInt(dst, int64(trunc))
	default:
		return errSkipPrimitive
	}
}

// assignSlice 按元素写入另一元素类型的切片。[]byte 仍走文本路径，不当成数字数组。
func assignSlice(dst, src reflect.Value) error {
	if src.Type().Elem().Kind() == reflect.Uint8 && dst.Type().Elem().Kind() != reflect.Uint8 {
		return errSkipPrimitive
	}
	if src.IsNil() {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	out := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
	for i := 0; i < src.Len(); i++ {
		elem := src.Index(i)
		if !elem.CanInterface() {
			return fmt.Errorf("cannot read slice element %d", i)
		}
		if err := assignField(out.Index(i), elem.Interface()); err != nil {
			return err
		}
	}
	dst.Set(out)
	return nil
}

// assignMap 按键和值分别转换后写入目标 map。
func assignMap(dst, src reflect.Value) error {
	if src.IsNil() {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}
	out := reflect.MakeMapWithSize(dst.Type(), src.Len())
	iter := src.MapRange()
	for iter.Next() {
		key := reflect.New(dst.Type().Key()).Elem()
		val := reflect.New(dst.Type().Elem()).Elem()
		if err := assignField(key, iter.Key().Interface()); err != nil {
			return err
		}
		if err := assignField(val, iter.Value().Interface()); err != nil {
			return err
		}
		out.SetMapIndex(key, val)
	}
	dst.Set(out)
	return nil
}

// asTime 将 time.Time / 字符串解析为墙钟 DATETIME。
func asTime(src any) (time.Time, error) {
	src = unwrapBytes(src)
	if isSQLNull(src) {
		return time.Time{}, nil
	}
	switch v := src.(type) {
	case time.Time:
		return record.NormalizeDateTime(v), nil
	case *time.Time:
		if v == nil {
			return time.Time{}, nil
		}
		return record.NormalizeDateTime(*v), nil
	case string:
		return record.ParseDateTime(v)
	default:
		return time.Time{}, fmt.Errorf("not a datetime: %T", src)
	}
}

// listArgs 把切片或数组拆成逐项绑定值。[]byte、[N]byte 和 driver.Valuer 仍是单个参数。
// 空切片返回 ok=true 且 items 为空，由调用方决定是否允许。
func listArgs(v any) (items []any, ok bool, err error) {
	if v == nil {
		return nil, false, nil
	}
	if _, isValuer := v.(driver.Valuer); isValuer {
		return nil, false, nil
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false, nil
		}
		if rv.CanInterface() {
			if _, isValuer := rv.Interface().(driver.Valuer); isValuer {
				return nil, false, nil
			}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false, nil
	}
	if rv.Type().Elem().Kind() == reflect.Uint8 {
		return nil, false, nil
	}
	items = make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		items[i], err = bindValue(rv.Index(i))
		if err != nil {
			return nil, true, fmt.Errorf("index %d: %w", i, err)
		}
	}
	return items, true, nil
}

// bindArg 把 Args 里的 Go 值收成驱动可绑定的值（含指针、Flag、时间）。
func bindArg(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	return bindValue(reflect.ValueOf(v))
}

// bindValue 解引用指针后交给 normalizeBound；nil 指针绑定为 SQL NULL。
func bindValue(v reflect.Value) (any, error) {
	if !v.IsValid() {
		return nil, nil
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil, nil
	}
	if v.CanInterface() {
		if vu, ok := v.Interface().(driver.Valuer); ok {
			val, err := vu.Value()
			if err != nil {
				return nil, err
			}
			return normalizeBound(val)
		}
	}
	return normalizeBound(v.Interface())
}

// normalizeBound 统一写出库的值：空 Flag→N，空 Decimal / 零时间→NULL，其余原样。
func normalizeBound(iface any) (any, error) {
	if iface == nil {
		return nil, nil
	}
	switch x := iface.(type) {
	case record.Flag:
		if x == "" {
			return record.FlagN.String(), nil
		}
		return x.String(), nil
	case record.Decimal:
		if x.Empty() {
			return nil, nil
		}
		return x.String(), nil
	case time.Time:
		if x.IsZero() {
			return nil, nil
		}
		return record.FormatDateTime(x), nil
	default:
		return iface, nil
	}
}
