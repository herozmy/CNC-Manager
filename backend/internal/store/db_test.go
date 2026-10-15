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

func TestMigrateScopesProgramToolsByVersion(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "version-tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("执行迁移失败：%v", err)
	}

	var versionColumn int
	if err := db.Get(&versionColumn, `SELECT COUNT(*) FROM pragma_table_info('program_tool')
		WHERE name = 'version_id' AND "notnull" = 1`); err != nil {
		t.Fatal(err)
	}
	if versionColumn != 1 {
		t.Fatalf("program_tool.version_id 列数量 = %d，期望 1", versionColumn)
	}

	var uniqueIndex int
	if err := db.Get(&uniqueIndex, `SELECT COUNT(*) FROM pragma_index_list('program_tool')
		WHERE name = 'ux_program_tool_version_seq' AND "unique" = 1`); err != nil {
		t.Fatal(err)
	}
	if uniqueIndex != 1 {
		t.Fatalf("版本内刀具行号唯一索引数量 = %d，期望 1", uniqueIndex)
	}
}
