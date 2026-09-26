package oracle

import (
	"io"
	"strings"
	"testing"
)

func TestConvertOracleIgnoresPlainValues(t *testing.T) {
	if _, ok, err := convertOracle("12"); ok || err != nil {
		t.Fatalf("string handled=%v err=%v", ok, err)
	}
	if _, ok, err := convertOracle(int64(1)); ok || err != nil {
		t.Fatalf("int handled=%v err=%v", ok, err)
	}
}

func TestReadOracleReader(t *testing.T) {
	text, err := readOracleReader(strings.NewReader("你好"), true)
	if err != nil || text != "你好" {
		t.Fatalf("clob: %#v %v", text, err)
	}
	blob, err := readOracleReader(strings.NewReader("ab"), false)
	if err != nil || string(blob.([]byte)) != "ab" {
		t.Fatalf("blob: %#v %v", blob, err)
	}
	empty, err := readOracleReader(nil, true)
	if err != nil || empty != "" {
		t.Fatalf("nil clob: %#v %v", empty, err)
	}
}

type closeReader struct {
	r      io.Reader
	closed bool
}

func (c *closeReader) Read(p []byte) (int, error) { return c.r.Read(p) }
func (c *closeReader) Close() error {
	c.closed = true
	return nil
}

func TestReadOracleReaderCloses(t *testing.T) {
	body := &closeReader{r: strings.NewReader("lob")}
	got, err := readOracleReader(body, true)
	if err != nil || got != "lob" || !body.closed {
		t.Fatalf("got %#v closed=%v err=%v", got, body.closed, err)
	}
}
