package record

import (
	"fmt"
	"strings"
	"time"
)

// Flag 表示平台库 VARCHAR(1) 开关列，取值仅为 Y 或 N，禁止用 bool 直接落库。
type Flag string

const (
	// FlagY 表示开关开启，对应库内 'Y'。
	FlagY Flag = "Y"
	// FlagN 表示开关关闭，对应库内 'N'。
	FlagN Flag = "N"
)

// Bool 将 Flag 转为 bool：仅 Y 为 true，其余为 false。
func (f Flag) Bool() bool { return f == FlagY }

// Valid 报告 f 是否为合法的 Y 或 N。
func (f Flag) Valid() bool { return f == FlagY || f == FlagN }

// String 返回 Flag 的库内文本，即 Y 或 N。
func (f Flag) String() string { return string(f) }

// FlagFromBool 把 bool 转成库内开关值：true 为 Y，false 为 N。
func FlagFromBool(v bool) Flag {
	if v {
		return FlagY
	}
	return FlagN
}

// ParseFlag 解析开关文本（大小写不敏感）；空串视为 N，非法值返回错误。
func ParseFlag(s string) (Flag, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "N":
		return FlagN, nil
	case "Y":
		return FlagY, nil
	default:
		return "", fmt.Errorf("dbx/record: invalid flag %q, want Y or N", s)
	}
}

// DateTimeLayout 是平台库 DATETIME 的统一文本格式（无时区，按墙钟写入）。
const DateTimeLayout = "2006-01-02 15:04:05"

// NormalizeDateTime 去掉亚秒，并把墙钟钉在 UTC，便于 DATETIME 往返比较。
// 不把 18:00 CST 改成 10:00 UTC，只统一 Location。
func NormalizeDateTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

// FormatDateTime 将时间格式化为 DATETIME 文本；零值返回空串。按墙钟写入，不换时区。
func FormatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return NormalizeDateTime(t).Format(DateTimeLayout)
}

// ParseDateTime 解析 DATETIME 文本，兼容 RFC3339、带毫秒与纯日期；空串得到零值。
func ParseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		DateTimeLayout,
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05.000000",
		"2006-01-02 15:04:05.999999999",
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	var last error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.UTC)
		if err == nil {
			return NormalizeDateTime(t), nil
		}
		last = err
	}
	return time.Time{}, fmt.Errorf("dbx/record: parse datetime %q: %w", s, last)
}

// Decimal 表示 NUMERIC / DECIMAL / 金额列，用字符串进出以避免 float64 精度丢失。
type Decimal string

// String 返回原始数字文本。
func (d Decimal) String() string { return string(d) }

// Empty 报告 Decimal 去掉首尾空白后是否为空。
func (d Decimal) Empty() bool { return strings.TrimSpace(string(d)) == "" }

// ParseDecimal 去掉首尾空白得到 Decimal；空串合法，不在此做精度截断。
func ParseDecimal(s string) Decimal {
	return Decimal(strings.TrimSpace(s))
}
