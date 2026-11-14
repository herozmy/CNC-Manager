package repo

import (
	"cnc-manager/internal/domain"
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestVersionToolsRoundTripIsolationAndRollback(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	p := newProgram(t, r, "TOOLS")
	a, _ := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "a.nc", "")
	b, _ := r.AddVersion(ctx, p.ID, blob("bb22", "utf-8"), "b.nc", "")
	// Seed the exact storage format used by v1 migration, including dictionary link.
	_, err := r.db.Exec(`INSERT INTO tool(id,tool_no,name,created_at,updated_at) VALUES(5,'LIB5','旧刀具','2026-11-14','2026-11-14')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.db.Exec(`INSERT INTO program_tool(program_id,version_id,seq,tool_id,tool_no,offset_no,tool_name,tool_dia_milli,corner_radius_milli,comp_amount_milli,spindle_speed,speed_mode,feed_milli,feed_mode,cut_depth_milli,coolant,machining_content,remark,custom_params_json)
 VALUES(?,?,9,5,'T01','H01','旧刀具',12345,800,-123,250,1,125,1,1250,3,'精车','保留备注','[{"name":"品牌","value":"旧品牌"}]')`, p.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := r.GetVersionTools(ctx, a.ID)
	if err != nil || len(before.Items) != 1 {
		t.Fatalf("%+v %v", before, err)
	}
	got := before.Items[0]
	if got.ToolDia != 12.345 || got.CompAmount != -.123 || got.Feed != .125 || got.ToolID == nil || *got.ToolID != 5 {
		t.Fatalf("旧数据读取错误 %+v", got)
	}
	id := int64(5)
	row := domain.ProgramToolInput{ToolID: &id, ToolNo: "T01", OffsetNo: "H01", ToolName: "旧刀具", ToolDia: 12.345, CornerRadius: .8, CompAmount: -.456, SpindleSpeed: 250, SpeedMode: 1, Feed: .125, FeedMode: 1, CutDepth: 1.25, Coolant: 3, MachiningContent: "精车", Remark: "保留备注", CustomParams: got.CustomParams}
	saved, err := r.ReplaceVersionTools(ctx, a.ID, domain.ToolTableInput{Items: []domain.ProgramToolInput{row, {ToolNo: "T02", CompAmount: .001}}, ExpectedRevision: before.Revision})
	if err != nil {
		t.Fatal(err)
	}
	want := got
	want.ID = saved.Items[0].ID
	want.Seq = 1
	want.CompAmount = -.456
	want.CompAmountMilli = -456
	if !reflect.DeepEqual(saved.Items[0], want) {
		t.Fatalf("字段丢失\n%+v\n%+v", saved.Items[0], want)
	}
	other, _ := r.GetVersionTools(ctx, b.ID)
	if len(other.Items) != 0 {
		t.Fatal("新版本应从空表开始")
	}
	// Overwriting NC bytes or switching current must not change tool tables.
	if _, err := r.OverwriteVersion(ctx, a.ID, blob("cc33", "utf-8"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetCurrentVersion(ctx, p.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := r.GetVersionTools(ctx, a.ID)
	if !reflect.DeepEqual(saved, unchanged) {
		t.Fatal("NC 更新影响了刀具表")
	}
	// A late FK failure must roll back DELETE and any preceding successful INSERTs.
	missing := int64(99999)
	_, err = r.ReplaceVersionTools(ctx, a.ID, domain.ToolTableInput{ExpectedRevision: saved.Revision, Items: []domain.ProgramToolInput{row, {ToolNo: "BAD", ToolID: &missing}}})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	unchanged, _ = r.GetVersionTools(ctx, a.ID)
	if !reflect.DeepEqual(saved, unchanged) {
		t.Fatal("失败留下半张表")
	}
	// A stale editor cannot erase the winning table.
	_, err = r.ReplaceVersionTools(ctx, a.ID, domain.ToolTableInput{ExpectedRevision: before.Revision, Items: []domain.ProgramToolInput{}})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	cleared, err := r.ReplaceVersionTools(ctx, a.ID, domain.ToolTableInput{ExpectedRevision: saved.Revision, Items: []domain.ProgramToolInput{}})
	if err != nil || len(cleared.Items) != 0 {
		t.Fatalf("%+v %v", cleared, err)
	}
	if err := r.DeleteProgram(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetVersionTools(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestVersionToolsConcurrentSave(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	p := newProgram(t, r, "CONCURRENT-TOOLS")
	v, _ := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "a.nc", "")
	initial, _ := r.GetVersionTools(ctx, v.ID)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, no := range []string{"T01", "T02"} {
		wg.Add(1)
		go func(no string) {
			defer wg.Done()
			_, err := r.ReplaceVersionTools(ctx, v.ID, domain.ToolTableInput{ExpectedRevision: initial.Revision, Items: []domain.ProgramToolInput{{ToolNo: no}}})
			results <- err
		}(no)
	}
	wg.Wait()
	close(results)
	ok, conflicts := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatalf("success %d conflict %d", ok, conflicts)
	}
}
