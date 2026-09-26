package db

import (
	"reflect"
	"testing"
	"time"

	"github.com/fluxsce/dbx/record"
)

type sampleRow struct {
	TenantID    string      `db:"tenantId,pk"`
	ProjectID   string      `db:"projectId,pk"`
	ProjectName string      `db:"projectName"`
	Note        string      `db:"noteText,omitempty"`
	ActiveFlag  record.Flag `db:"activeFlag"`
	AddTime     time.Time   `db:"addTime,noupdate"`
	Skip        string      `db:"-"`
	unexported  string
}

func TestParseColsAndColumns(t *testing.T) {
	cols, err := parseCols(reflect.TypeOf(sampleRow{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 6 {
		t.Fatalf("len %d", len(cols))
	}
	if !cols[0].pk || cols[0].column != "tenantId" {
		t.Fatalf("%+v", cols[0])
	}
	s, err := Columns(&sampleRow{})
	if err != nil {
		t.Fatal(err)
	}
	if s != "tenantId, projectId, projectName, noteText, activeFlag, addTime" {
		t.Fatalf("got %q", s)
	}
}

type EmbedPK struct {
	TenantID  string `db:"tenantId,pk"`
	ProjectID string `db:"projectId,pk"`
}

type embedRow struct {
	EmbedPK
	ProjectName string `db:"projectName"`
}

func TestParseColsEmbedded(t *testing.T) {
	cols, err := parseCols(reflect.TypeOf(embedRow{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 || cols[0].column != "tenantId" || !cols[0].pk {
		t.Fatalf("%+v", cols)
	}
	s, err := Columns(&embedRow{})
	if err != nil {
		t.Fatal(err)
	}
	if s != "tenantId, projectId, projectName" {
		t.Fatalf("got %q", s)
	}
}

func TestStripColumnName(t *testing.T) {
	if stripColumnName(`AI_PROJECT."tenantId"`) != "tenantId" {
		t.Fatal("quoted")
	}
	if stripColumnName("AI_DEMO.tenantId") != "tenantId" {
		t.Fatal("prefix")
	}
}

func TestLookupColumnFoldCase(t *testing.T) {
	sch, err := loadSchema(reflect.TypeOf(sampleRow{}))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := lookupColumn(sch.byName, sch.byLower, "TENANTID")
	if !ok || c.column != "tenantId" {
		t.Fatalf("oracle fold %+v %v", c, ok)
	}
	c, ok = lookupColumn(sch.byName, sch.byLower, "tenantid")
	if !ok || c.column != "tenantId" {
		t.Fatalf("pg fold %+v %v", c, ok)
	}
}

func TestSchemaCache(t *testing.T) {
	a, err := loadSchema(reflect.TypeOf(sampleRow{}))
	if err != nil {
		t.Fatal(err)
	}
	b, err := loadSchema(reflect.TypeOf(&sampleRow{}))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("schema should be cached by struct type")
	}
}

func TestValidIdent(t *testing.T) {
	if !validIdent("AI_PROJECT") || !validIdent("tenantId") {
		t.Fatal("ok")
	}
	if validIdent("AI PROJECT") || validIdent("a;drop") {
		t.Fatal("bad")
	}
}

func TestQuoteOrderBy(t *testing.T) {
	sess := &DB{dial: qmarkDialect{}}
	s, err := sess.quoteOrderBy("tenantId, demoId DESC")
	if err != nil || s != "tenantId, demoId DESC" {
		t.Fatalf("%q %v", s, err)
	}
	if _, err := sess.quoteOrderBy("id;drop"); err == nil {
		t.Fatal("inject")
	}
	if _, err := sess.quoteOrderBy("id ASC extra"); err == nil {
		t.Fatal("too many tokens")
	}
	if _, err := sess.quoteOrderBy("id NULLS"); err == nil {
		t.Fatal("bad direction")
	}
}
