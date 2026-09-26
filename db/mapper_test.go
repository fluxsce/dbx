package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fluxsce/dbx/db"
	_ "github.com/fluxsce/dbx/driver/sqlite"
	"github.com/fluxsce/dbx/record"
)

type demoRow struct {
	TenantID   string         `db:"tenantId,pk"`
	DemoID     string         `db:"demoId,pk"`
	DemoName   string         `db:"demoName"`
	ActiveFlag record.Flag    `db:"activeFlag"`
	Price      record.Decimal `db:"price,omitempty"`
	AddTime    time.Time      `db:"addTime,noupdate"`
	EditTime   time.Time      `db:"editTime"`
}

func TestInsertUpdateDeleteGetSelect(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL,
		demoId TEXT NOT NULL,
		demoName TEXT NOT NULL,
		activeFlag TEXT NOT NULL,
		price TEXT,
		addTime TEXT,
		editTime TEXT,
		PRIMARY KEY (tenantId, demoId)
	)`, nil); err != nil {
		t.Fatal(err)
	}

	add := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	row := demoRow{
		TenantID:   "default",
		DemoID:     "d1",
		DemoName:   "销售报表",
		ActiveFlag: record.FlagY,
		Price:      record.ParseDecimal("19.90"),
		AddTime:    add,
		EditTime:   add,
	}
	if err := d.Insert(ctx, "AI_DEMO", &row); err != nil {
		t.Fatal(err)
	}

	cols, err := db.Columns(&demoRow{})
	if err != nil {
		t.Fatal(err)
	}
	quoted, err := d.QuoteColumns(&demoRow{})
	if err != nil {
		t.Fatal(err)
	}
	if quoted == cols || !strings.Contains(quoted, `"tenantId"`) {
		t.Fatalf("QuoteColumns %q", quoted)
	}
	var got demoRow
	if err := d.Get(ctx, &got, `SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId AND demoId=@demoId`, db.Args{
		"tenantId": "default", "demoId": "d1",
	}); err != nil {
		t.Fatal(err)
	}
	if got.DemoName != "销售报表" || !got.ActiveFlag.Bool() || got.Price.String() != "19.90" {
		t.Fatalf("%+v", got)
	}

	row.DemoName = "改名"
	row.ActiveFlag = record.FlagN
	row.EditTime = add.Add(time.Hour)
	row.AddTime = time.Time{}
	pk, err := d.PK(&row)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Update(ctx, "AI_DEMO", &row, pk); err != nil {
		t.Fatal(err)
	}
	got = demoRow{}
	if err := d.Get(ctx, &got, `SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId AND demoId=@demoId`, db.Args{
		"tenantId": "default", "demoId": "d1",
	}); err != nil {
		t.Fatal(err)
	}
	if got.DemoName != "改名" || got.ActiveFlag.Bool() {
		t.Fatalf("%+v", got)
	}
	if got.AddTime.UTC().Format(record.DateTimeLayout) != "2026-08-21 10:00:00" {
		t.Fatalf("addTime overwritten: %v", got.AddTime)
	}

	row2 := row
	row2.DemoID = "d2"
	row2.DemoName = "第二"
	row2.AddTime = add
	row2.EditTime = add
	if err := d.Insert(ctx, "AI_DEMO", &row2); err != nil {
		t.Fatal(err)
	}

	err = d.Tx(ctx, func(tx *db.DB) error {
		var list []demoRow
		if err := tx.Select(ctx, &list, `SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId ORDER BY demoId`, db.Args{"tenantId": "default"}); err != nil {
			return err
		}
		if len(list) != 2 {
			t.Fatalf("%+v", list)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var page []demoRow
	total, err := d.SelectPage(ctx, &page,
		`SELECT COUNT(*) FROM AI_DEMO WHERE tenantId=@tenantId`,
		`SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId ORDER BY demoId`,
		db.Page{Page: 1, PageSize: 1},
		db.Args{"tenantId": "default"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(page) != 1 || page[0].DemoID != "d1" {
		t.Fatalf("page total=%d list=%+v", total, page)
	}

	if err := d.Delete(ctx, "AI_DEMO", mustPK(t, d, &demoRow{TenantID: "default", DemoID: "d2"})); err != nil {
		t.Fatal(err)
	}
	err = d.Get(ctx, &got, `SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId AND demoId=@demoId`, db.Args{
		"tenantId": "default", "demoId": "d2",
	})
	if err != sql.ErrNoRows {
		t.Fatalf("want ErrNoRows got %v", err)
	}
}

type nullableRow struct {
	TenantID string     `db:"tenantId,pk"`
	DemoID   string     `db:"demoId,pk"`
	Note     *string    `db:"noteText"`
	Price    *string    `db:"price"`
	AddTime  *time.Time `db:"addTime"`
	EditTime time.Time  `db:"editTime"`
}

func TestNullPointerAndZeroTime(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL,
		demoId TEXT NOT NULL,
		noteText TEXT,
		price TEXT,
		addTime TEXT,
		editTime TEXT,
		PRIMARY KEY (tenantId, demoId)
	)`, nil); err != nil {
		t.Fatal(err)
	}

	row := nullableRow{TenantID: "default", DemoID: "n1"}
	if err := d.Insert(ctx, "AI_DEMO", &row); err != nil {
		t.Fatal(err)
	}
	cols, err := db.Columns(&nullableRow{})
	if err != nil {
		t.Fatal(err)
	}
	var got nullableRow
	if err := d.Get(ctx, &got, `SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId AND demoId=@demoId`, db.Args{
		"tenantId": "default", "demoId": "n1",
	}); err != nil {
		t.Fatal(err)
	}
	if got.Note != nil || got.Price != nil || got.AddTime != nil || !got.EditTime.IsZero() {
		t.Fatalf("want all NULL/zero got %+v", got)
	}

	note := "备注"
	tm := time.Date(2026, 8, 21, 18, 0, 0, 0, time.FixedZone("CST", 8*3600))
	row.Note = &note
	row.AddTime = &tm
	row.EditTime = tm
	if err := d.Update(ctx, "AI_DEMO", &row, mustPK(t, d, &row)); err != nil {
		t.Fatal(err)
	}
	got = nullableRow{}
	if err := d.Get(ctx, &got, `SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId AND demoId=@demoId`, db.Args{
		"tenantId": "default", "demoId": "n1",
	}); err != nil {
		t.Fatal(err)
	}
	if got.Note == nil || *got.Note != "备注" {
		t.Fatalf("note %+v", got.Note)
	}
	if got.AddTime == nil || record.FormatDateTime(*got.AddTime) != "2026-08-21 18:00:00" {
		t.Fatalf("addTime %+v", got.AddTime)
	}
	if record.FormatDateTime(got.EditTime) != "2026-08-21 18:00:00" {
		t.Fatalf("editTime %v", got.EditTime)
	}

	row.Note = nil
	row.AddTime = nil
	row.EditTime = time.Time{}
	if err := d.Update(ctx, "AI_DEMO", &row, mustPK(t, d, &row)); err != nil {
		t.Fatal(err)
	}
	got = nullableRow{}
	if err := d.Get(ctx, &got, `SELECT `+cols+` FROM AI_DEMO WHERE tenantId=@tenantId AND demoId=@demoId`, db.Args{
		"tenantId": "default", "demoId": "n1",
	}); err != nil {
		t.Fatal(err)
	}
	if got.Note != nil || got.AddTime != nil || !got.EditTime.IsZero() {
		t.Fatalf("cleared NULL %+v", got)
	}
}

func TestSelectResetsSlice(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL, demoId TEXT NOT NULL, demoName TEXT NOT NULL,
		activeFlag TEXT NOT NULL, price TEXT, addTime TEXT, editTime TEXT,
		PRIMARY KEY (tenantId, demoId)
	)`, nil); err != nil {
		t.Fatal(err)
	}
	row := demoRow{TenantID: "default", DemoID: "a", DemoName: "A", ActiveFlag: record.FlagY}
	if err := d.Insert(ctx, "AI_DEMO", &row); err != nil {
		t.Fatal(err)
	}
	list := []demoRow{{DemoID: "stale"}}
	if err := d.Select(ctx, &list, `SELECT tenantId, demoId, demoName, activeFlag, price, addTime, editTime FROM AI_DEMO`, nil); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].DemoID != "a" {
		t.Fatalf("%+v", list)
	}
}

type DemoPK struct {
	TenantID  string `db:"tenantId,pk"`
	ProjectID string `db:"projectId,pk"`
}

type embedDemo struct {
	DemoPK
	DemoName   string      `db:"demoName"`
	ActiveFlag record.Flag `db:"activeFlag"`
}

func TestEmbeddedInsertGet(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL, projectId TEXT NOT NULL, demoName TEXT NOT NULL, activeFlag TEXT NOT NULL,
		PRIMARY KEY (tenantId, projectId)
	)`, nil); err != nil {
		t.Fatal(err)
	}
	row := embedDemo{DemoPK: DemoPK{TenantID: "default", ProjectID: "p1"}, DemoName: "嵌套", ActiveFlag: record.FlagY}
	if err := d.Insert(ctx, "AI_DEMO", &row); err != nil {
		t.Fatal(err)
	}
	var got embedDemo
	if err := d.Get(ctx, &got, `SELECT tenantid, projectId, demoName, activeFlag FROM AI_DEMO`, nil); err != nil {
		t.Fatal(err)
	}
	if got.TenantID != "default" || got.ProjectID != "p1" || got.DemoName != "嵌套" {
		t.Fatalf("%+v", got)
	}
}

type noPKRow struct {
	TenantID string `db:"tenantId"`
	Msg      string `db:"msg"`
}

func TestUpdateDeleteCustomWhere(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL, demoId TEXT NOT NULL, demoName TEXT NOT NULL,
		activeFlag TEXT NOT NULL, price TEXT, addTime TEXT, editTime TEXT,
		PRIMARY KEY (tenantId, demoId)
	)`, nil); err != nil {
		t.Fatal(err)
	}
	row := demoRow{TenantID: "default", DemoID: "d1", DemoName: "旧", ActiveFlag: record.FlagY}
	if err := d.Insert(ctx, "AI_DEMO", &row); err != nil {
		t.Fatal(err)
	}
	cond, err := d.PK(&row)
	if err != nil {
		t.Fatal(err)
	}
	row.DemoID = "d2"
	row.DemoName = "新主键"
	if err := d.Update(ctx, "AI_DEMO", &row, cond); err != nil {
		t.Fatal(err)
	}
	var got demoRow
	if err := d.Get(ctx, &got, `SELECT tenantId, demoId, demoName, activeFlag, price, addTime, editTime FROM AI_DEMO WHERE demoId=@demoId`, db.Args{"demoId": "d2"}); err != nil {
		t.Fatal(err)
	}
	if got.DemoName != "新主键" {
		t.Fatalf("%+v", got)
	}
	if err := d.Get(ctx, &got, `SELECT tenantId, demoId, demoName, activeFlag, price, addTime, editTime FROM AI_DEMO WHERE demoId=@demoId`, db.Args{"demoId": "d1"}); err != sql.ErrNoRows {
		t.Fatalf("old pk still there: %v", err)
	}

	if err := d.Delete(ctx, "AI_DEMO", db.Where(`tenantId=@tenantId AND demoId=@demoId`, db.Args{"tenantId": "default", "demoId": "d2"})); err != nil {
		t.Fatal(err)
	}
	if err := d.Get(ctx, &got, `SELECT tenantId FROM AI_DEMO WHERE demoId=@demoId`, db.Args{"demoId": "d2"}); err != sql.ErrNoRows {
		t.Fatalf("want deleted %v", err)
	}

	if _, err := d.Exec(ctx, `CREATE TABLE AI_LOG (tenantId TEXT NOT NULL, msg TEXT NOT NULL)`, nil); err != nil {
		t.Fatal(err)
	}
	log := noPKRow{TenantID: "default", Msg: "a"}
	if err := d.Insert(ctx, "AI_LOG", &log); err != nil {
		t.Fatal(err)
	}
	log.Msg = "b"
	if err := d.Update(ctx, "AI_LOG", &log, db.Where(`tenantId=@tenantId AND msg=@oldMsg`, db.Args{"tenantId": "default", "oldMsg": "a"})); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(ctx, "AI_LOG", db.Where(`WHERE tenantId=@tenantId AND msg=@msg`, db.Args{"tenantId": "default", "msg": "b"})); err != nil {
		t.Fatal(err)
	}
	if err := d.Update(ctx, "AI_LOG", &log, db.Cond{}); err == nil {
		t.Fatal("empty cond")
	}
	if err := d.Delete(ctx, "AI_LOG", db.Cond{}); err == nil {
		t.Fatal("empty cond")
	}
}

func TestInsertBatch(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL, demoId TEXT NOT NULL, demoName TEXT NOT NULL,
		activeFlag TEXT NOT NULL, price TEXT, addTime TEXT, editTime TEXT,
		PRIMARY KEY (tenantId, demoId)
	)`, nil); err != nil {
		t.Fatal(err)
	}
	rows := []demoRow{
		{TenantID: "default", DemoID: "b1", DemoName: "一批1", ActiveFlag: record.FlagY},
		{TenantID: "default", DemoID: "b2", DemoName: "一批2", ActiveFlag: record.FlagN},
	}
	if err := d.Insert(ctx, "AI_DEMO", rows); err != nil {
		t.Fatal(err)
	}
	ptrs := []*demoRow{
		{TenantID: "default", DemoID: "b3", DemoName: "一批3", ActiveFlag: record.FlagY},
	}
	if err := d.Insert(ctx, "AI_DEMO", ptrs); err != nil {
		t.Fatal(err)
	}
	var list []demoRow
	if err := d.Select(ctx, &list, `SELECT tenantId, demoId, demoName, activeFlag, price, addTime, editTime FROM AI_DEMO ORDER BY demoId`, nil); err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].DemoID != "b1" || list[2].DemoName != "一批3" {
		t.Fatalf("%+v", list)
	}
}

func TestEachAndEachTable(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "t.db")
	d, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL, demoId TEXT NOT NULL, demoName TEXT NOT NULL,
		activeFlag TEXT NOT NULL, price TEXT, addTime TEXT, editTime TEXT,
		PRIMARY KEY (tenantId, demoId)
	)`, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.Insert(ctx, "AI_DEMO", []demoRow{
		{TenantID: "default", DemoID: "e1", DemoName: "流1", ActiveFlag: record.FlagY},
		{TenantID: "default", DemoID: "e2", DemoName: "流2", ActiveFlag: record.FlagN},
		{TenantID: "default", DemoID: "e3", DemoName: "流3", ActiveFlag: record.FlagY},
	}); err != nil {
		t.Fatal(err)
	}

	var got []string
	var row demoRow
	if err := d.EachTable(ctx, &row, "AI_DEMO", "demoId", func() error {
		got = append(got, row.DemoID+"/"+row.DemoName)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "e1/流1" || got[2] != "e3/流3" {
		t.Fatalf("%v", got)
	}

	n := 0
	err = d.Each(ctx, &row, `SELECT tenantId, demoId, demoName, activeFlag, price, addTime, editTime FROM AI_DEMO ORDER BY demoId`, nil, func() error {
		n++
		if n == 2 {
			return context.Canceled
		}
		return nil
	})
	if err != context.Canceled || n != 2 {
		t.Fatalf("stop n=%d err=%v", n, err)
	}

	if err := d.EachTable(ctx, &row, "AI_DEMO", "demoId;drop", func() error { return nil }); err == nil {
		t.Fatal("bad order by")
	}
	if err := d.Each(ctx, &row, `SELECT demoId FROM AI_DEMO`, nil, nil); err == nil {
		t.Fatal("fn required")
	}

	emptyDSN := filepath.Join(t.TempDir(), "empty.db")
	empty, err := db.Open(context.Background(), db.Config{Driver: "sqlite", DSN: emptyDSN})
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	if _, err := empty.Exec(ctx, `CREATE TABLE AI_DEMO (
		tenantId TEXT NOT NULL, demoId TEXT NOT NULL, demoName TEXT NOT NULL,
		activeFlag TEXT NOT NULL, price TEXT, addTime TEXT, editTime TEXT,
		PRIMARY KEY (tenantId, demoId)
	)`, nil); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := empty.EachTable(ctx, &row, "AI_DEMO", "", func() error {
		calls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("empty table called fn %d times", calls)
	}
}

func mustPK(t *testing.T, d *db.DB, row any) db.Cond {
	t.Helper()
	c, err := d.PK(row)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
