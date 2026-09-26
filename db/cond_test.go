package db

import "testing"

func TestWhereAndEmptyCond(t *testing.T) {
	c := Where("  WHERE tenantId=@tenantId", Args{"tenantId": "default"})
	if c.SQL != "tenantId=@tenantId" || c.Args["tenantId"] != "default" {
		t.Fatalf("%+v", c)
	}
	if _, err := (Cond{}).normalized(); err == nil {
		t.Fatal("empty cond")
	}
}
