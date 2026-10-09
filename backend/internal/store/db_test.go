package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateAddsToolCustomParamsColumn(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for range 2 {
		if err := Migrate(context.Background(), db); err != nil {
			t.Fatalf("执行迁移失败：%v", err)
		}
	}

	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM pragma_table_info('program_tool')
		WHERE name = 'custom_params_json' AND "notnull" = 1`); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("program_tool.custom_params_json 列数量 = %d，期望 1", count)
	}
}
