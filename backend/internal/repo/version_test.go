package repo

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"cnc-manager/internal/domain"
)

func blob(sha, enc string) domain.BlobInput {
	return domain.BlobInput{SHA256: sha, SizeBytes: 10, Encoding: enc, RelPath: sha[:2] + "/" + sha + ".nc"}
}

func TestOverwriteVersionConcurrentCheck(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	p := newProgram(t, r, "CONFLICT")
	v, err := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "a.nc", "")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for _, sha := range []string{"bb22", "cc33"} {
		wg.Add(1)
		go func(sha string) {
			defer wg.Done()
			<-start
			_, err := r.OverwriteVersion(ctx, v.ID, blob(sha, "utf-8"), &v.SHA256)
			results <- err
		}(sha)
	}
	close(start)
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
		t.Fatalf("成功=%d 冲突=%d", ok, conflicts)
	}
	var count int
	if err := r.db.Get(&count, "SELECT COUNT(*) FROM nc_blob"); err != nil || count != 2 {
		t.Fatalf("冲突不应写入 blob: %d %v", count, err)
	}
	got, err := r.GetVersion(ctx, v.ID)
	if err != nil || (got.SHA256 != "bb22" && got.SHA256 != "cc33") {
		t.Fatalf("%+v %v", got, err)
	}
}

func newProgram(t *testing.T, r *Repo, drawingNo string) *domain.Program {
	t.Helper()
	ctx := context.Background()
	d := seedDrawing(t, r, drawingNo)
	op, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10})
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "O1"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAddVersionIncrementsAndSetsCurrent(t *testing.T) {
	r := newTestRepo(t)
	ctx := domain.WithUser(context.Background(), domain.User{Username: "bob"})
	p := newProgram(t, r, "D-1")

	v1, err := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "a.nc", " 首版 ")
	if err != nil || v1.VersionNo != 1 || !v1.IsCurrent || v1.ChangeNote != "首版" || v1.CreatedBy != "bob" || v1.FileName != "a.nc" {
		t.Fatalf("v1 异常: %+v %v", v1, err)
	}
	v2, _ := r.AddVersion(ctx, p.ID, blob("bb22", "gbk"), "b.nc", "")
	if v2.VersionNo != 2 || !v2.IsCurrent || v2.Encoding != "gbk" {
		t.Fatalf("v2 异常: %+v", v2)
	}
	list, _ := r.ListVersions(ctx, p.ID)
	if len(list) != 2 || list[0].ID != v2.ID || list[0].IsCurrent != true || list[1].IsCurrent {
		t.Fatalf("列表异常: %+v", list)
	}
	prog, _ := r.GetProgram(ctx, p.ID)
	if prog.CurrentVersionID == nil || *prog.CurrentVersionID != v2.ID || *prog.CurrentVersionNo != 2 || prog.VersionCount != 2 {
		t.Fatalf("程序当前版本异常: %+v", prog)
	}
	if _, err := r.AddVersion(ctx, 999, blob("cc33", "utf-8"), "x", ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("不存在的程序应 NotFound，得到 %v", err)
	}
	if _, err := r.AddVersion(ctx, p.ID, blob("dd44", "utf-8"), "x", strings.Repeat("a", 501)); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("过长变更说明应被拒绝")
	}
	var n int
	_ = r.db.Get(&n, `SELECT COUNT(*) FROM nc_blob WHERE sha256 = 'dd44'`)
	if n != 0 {
		t.Fatal("失败的事务不应留下 blob 记录")
	}
}

func TestSameContentSharesBlobButKeepsOwnFileName(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	p1 := newProgram(t, r, "D-1")
	p2 := newProgram(t, r, "D-2")
	a, _ := r.AddVersion(ctx, p1.ID, blob("ee55", "utf-8"), "first.nc", "")
	b, err := r.AddVersion(ctx, p2.ID, blob("ee55", "utf-8"), "second.nc", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.FileName != "first.nc" || b.FileName != "second.nc" {
		t.Fatalf("文件名应跟版本走: %q %q", a.FileName, b.FileName)
	}
	var n int
	_ = r.db.Get(&n, `SELECT COUNT(*) FROM nc_blob`)
	if n != 1 {
		t.Fatalf("相同内容应只有一条 blob，得到 %d", n)
	}
}

func TestSetCurrentVersionRollback(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	p := newProgram(t, r, "D-1")
	other := newProgram(t, r, "D-2")
	v1, _ := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "a.nc", "")
	_, _ = r.AddVersion(ctx, p.ID, blob("bb22", "utf-8"), "b.nc", "")
	ov, _ := r.AddVersion(ctx, other.ID, blob("cc33", "utf-8"), "c.nc", "")

	got, err := r.SetCurrentVersion(ctx, p.ID, v1.ID)
	if err != nil || *got.CurrentVersionNo != 1 {
		t.Fatalf("回滚失败: %+v %v", got, err)
	}
	if _, err := r.SetCurrentVersion(ctx, p.ID, ov.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("别的程序的版本应被拒绝，得到 %v", err)
	}
	if _, err := r.SetCurrentVersion(ctx, p.ID, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("不存在的版本应 NotFound")
	}
	if _, err := r.SetCurrentVersion(ctx, 999, v1.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("不存在的程序应 NotFound")
	}
}

func TestOverwriteVersion(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	p := newProgram(t, r, "D-1")
	v1, _ := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "a.nc", "")
	v2, _ := r.AddVersion(ctx, p.ID, blob("bb22", "utf-8"), "b.nc", "")

	got, err := r.OverwriteVersion(ctx, v1.ID, blob("ff66", "gbk"), nil)
	if err != nil || got.SHA256 != "ff66" || got.Encoding != "gbk" || got.VersionNo != 1 || got.FileName != "a.nc" {
		t.Fatalf("覆盖异常: %+v %v", got, err)
	}
	other, _ := r.GetVersion(ctx, v2.ID)
	if other.SHA256 != "bb22" {
		t.Fatal("覆盖不应影响其他版本")
	}
	list, _ := r.ListVersions(ctx, p.ID)
	if len(list) != 2 {
		t.Fatal("覆盖不应新增版本")
	}
	if _, err := r.OverwriteVersion(ctx, 999, blob("ab12", "utf-8"), nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("不存在的版本应 NotFound")
	}
}

func TestGetVersionFileAndDeleteCascade(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	p := newProgram(t, r, "D-1")
	v, _ := r.AddVersion(ctx, p.ID, blob("aa11", "gbk"), "a.nc", "")
	f, err := r.GetVersionFile(ctx, v.ID)
	if err != nil || f.RelPath != "aa/aa11.nc" || f.Encoding != "gbk" || !f.IsCurrent || f.ProgramID != p.ID {
		t.Fatalf("文件信息异常: %+v %v", f, err)
	}
	if _, err := r.GetVersionFile(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("不存在应 NotFound")
	}
	if err := r.DeleteProgram(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetVersion(ctx, v.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("删除程序应级联删除版本")
	}
	if _, err := r.ListVersions(ctx, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("已删除程序的版本列表应 NotFound")
	}
}

func TestCatalogDeleteCascadePreservesSharedBlob(t *testing.T) {
	for _, level := range []string{"drawing", "operation", "program"} {
		t.Run(level, func(t *testing.T) {
			r := newTestRepo(t)
			ctx := context.Background()
			p := newProgram(t, r, "DELETE")
			keep := newProgram(t, r, "KEEP")
			v, err := r.AddVersion(ctx, p.ID, blob("aa11", "gbk"), "delete.nc", "")
			if err != nil {
				t.Fatal(err)
			}
			kept, err := r.AddVersion(ctx, keep.ID, blob("aa11", "gbk"), "keep.nc", "")
			if err != nil {
				t.Fatal(err)
			}
			op, err := r.GetOperation(ctx, p.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			switch level {
			case "drawing":
				err = r.DeleteDrawing(ctx, op.DrawingID)
			case "operation":
				err = r.DeleteOperation(ctx, op.ID)
			case "program":
				err = r.DeleteProgram(ctx, p.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.GetProgram(ctx, p.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("program remains: %v", err)
			}
			if _, err := r.GetVersionFile(ctx, v.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("version remains: %v", err)
			}
			f, err := r.GetVersionFile(ctx, kept.ID)
			if err != nil || f.FileName != "keep.nc" || f.RelPath != "aa/aa11.nc" {
				t.Fatalf("shared file damaged: %+v %v", f, err)
			}
			if _, err := r.GetProgram(ctx, keep.ID); err != nil {
				t.Fatal(err)
			}
			if level != "drawing" {
				if _, err := r.GetDrawing(ctx, op.DrawingID); err != nil {
					t.Fatal(err)
				}
			}
			if level == "program" {
				if _, err := r.GetOperation(ctx, op.ID); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
