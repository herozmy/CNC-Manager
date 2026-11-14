package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cnc-manager/internal/domain"
)

type fixture struct {
	h      http.Handler
	ck     *http.Cookie
	progID int64
	path   string // /api/programs/{id}
}

func TestOverwriteContentRejectsStaleDraft(t *testing.T) {
	f := newFixture(t, 1<<20)
	v := f.uploadOK(t, "part.nc", []byte("O1\r\nM30\r\n"))
	path := fmt.Sprintf("/api/versions/%d/content", v.ID)
	var before domain.VersionContent
	decode(t, f.get(t, path), &before)
	if before.SHA256 != v.SHA256 {
		t.Fatal("读取内容时缺少对应的校验值")
	}
	body, _ := json.Marshal(domain.ContentInput{Content: "O1\r\nG00 X2\r\nM30\r\n", ExpectedSHA256: &before.SHA256})
	first := do(t, f.h, "PUT", path, string(body), f.ck)
	if first.Code != 200 {
		t.Fatal(first.Body.String())
	}
	body, _ = json.Marshal(domain.ContentInput{Content: "O1\r\nG00 X3\r\nM30\r\n", ExpectedSHA256: &before.SHA256})
	stale := do(t, f.h, "PUT", path, string(body), f.ck)
	if stale.Code != 409 {
		t.Fatalf("旧草稿应拒绝: %d %s", stale.Code, stale.Body.String())
	}
	var after domain.VersionContent
	decode(t, f.get(t, path), &after)
	if !strings.Contains(after.Content, "X2") || after.SHA256 == before.SHA256 {
		t.Fatalf("获胜内容不应被覆盖: %+v", after)
	}
	// 冲突草稿仍可另存，既有版本保留。
	if rec := do(t, f.h, "POST", f.path+"/versions/content", string(body), f.ck); rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
}

func TestSaveLargeContentAndLimit(t *testing.T) {
	f := newFixture(t, 2<<20)
	content := strings.Repeat("G01 X1.25 Y2.50 F100\r\n", 60000)
	if len(content) <= 1<<20 {
		t.Fatal("fixture must exceed old JSON limit")
	}
	body, _ := json.Marshal(domain.ContentInput{Content: content, Encoding: "utf-8"})
	response := do(t, f.h, "POST", f.path+"/versions/content", string(body), f.ck)
	if response.Code != 201 {
		t.Fatalf("大文件保存失败: %d %s", response.Code, response.Body.String())
	}
	var saved domain.VersionSaveResult
	decode(t, response, &saved)
	if got := f.get(t, fmt.Sprintf("/api/versions/%d/download", saved.ID)).Body.String(); got != content {
		t.Fatal("大文件内容或换行改变")
	}
	body, _ = json.Marshal(domain.ContentInput{Content: strings.Repeat("X", (2<<20)+1)})
	if rec := do(t, f.h, "POST", f.path+"/versions/content", string(body), f.ck); rec.Code != 413 {
		t.Fatalf("超出文件上限应拒绝: %d", rec.Code)
	}
}

func TestDiffPaginationAndChangesOnly(t *testing.T) {
	f := newFixture(t, 1<<20)
	a := f.uploadOK(t, "a.nc", []byte("O1\n"+strings.Repeat("G00 X1\n", 230)+"M30\n"))
	b := f.uploadOK(t, "b.nc", []byte("O1\n"+strings.Repeat("G01 X2\n", 230)+"M30\n"))
	path := fmt.Sprintf("/api/versions/%d/diff?with=%d", a.ID, b.ID)
	var page domain.DiffResult
	decode(t, f.get(t, path+"&limit=200&offset=200&changesOnly=1"), &page)
	if page.TotalLines != 230 || page.TotalChanges != 230 || len(page.Lines) != 30 || *page.Lines[0].LeftNo != 202 {
		t.Fatalf("分页异常: total=%d changes=%d lines=%d", page.TotalLines, page.TotalChanges, len(page.Lines))
	}
	decode(t, f.get(t, path+"&limit=200&changesOnly=0"), &page)
	if page.TotalLines != 232 || len(page.Lines) != 200 || page.Lines[0].Type != "same" {
		t.Fatal("完整对比分页错误")
	}
	for _, suffix := range []string{"&limit=501", "&limit=0", "&limit=200&offset=-1"} {
		if rec := f.get(t, path+suffix); rec.Code != 400 {
			t.Fatalf("非法分页未被拒绝: %s", suffix)
		}
	}
}

func newFixture(t *testing.T, maxBytes int64) *fixture {
	t.Helper()
	h, _ := newTestServerMax(t, maxBytes)
	ck := setupAdmin(t, h)
	var d domain.Drawing
	decode(t, do(t, h, "POST", "/api/drawings", `{"drawingNo":"D-1"}`, ck), &d)
	var o domain.Operation
	decode(t, do(t, h, "POST", fmt.Sprintf("/api/drawings/%d/operations", d.ID), `{"opNo":10}`, ck), &o)
	var p domain.Program
	decode(t, do(t, h, "POST", fmt.Sprintf("/api/operations/%d/programs", o.ID), `{"programNo":"O1234"}`, ck), &p)
	return &fixture{h: h, ck: ck, progID: p.ID, path: fmt.Sprintf("/api/programs/%d", p.ID)}
}

func (f *fixture) upload(t *testing.T, name string, content []byte, note string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if name != "" {
		fw, _ := mw.CreateFormFile("file", name)
		_, _ = fw.Write(content)
	}
	if note != "" {
		_ = mw.WriteField("changeNote", note)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", f.path+"/versions", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", "http://example.com")
	req.AddCookie(f.ck)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) uploadOK(t *testing.T, name string, content []byte) domain.UploadVersionResult {
	t.Helper()
	rec := f.upload(t, name, content, "")
	if rec.Code != 201 {
		t.Fatalf("上传失败 %d: %s", rec.Code, rec.Body.String())
	}
	var res domain.UploadVersionResult
	decode(t, rec, &res)
	return res
}

func (f *fixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, f.h, "GET", path, "", f.ck)
}

const sampleNC = "O1234 (DEMO)\nG21 G40 G80\nT0101\nG00 X50 Z2\nM30\n"

func TestVersionEndpointsRequireLogin(t *testing.T) {
	h, _ := newTestServer(t)
	for _, c := range []struct{ m, p string }{
		{"GET", "/api/programs/1/versions"}, {"POST", "/api/programs/1/versions"}, {"POST", "/api/programs/1/versions/content"},
		{"PUT", "/api/programs/1/current-version"}, {"GET", "/api/versions/1/download"}, {"GET", "/api/versions/1/content"},
		{"PUT", "/api/versions/1/content"}, {"GET", "/api/versions/1/diff?with=2"}, {"POST", "/api/nc/parse"},
	} {
		if rec := do(t, h, c.m, c.p, "{}"); rec.Code != 401 {
			t.Errorf("%s %s 未登录应 401，得到 %d", c.m, c.p, rec.Code)
		}
	}
}

func TestUploadParsesToolsAndSetsCurrent(t *testing.T) {
	f := newFixture(t, 1<<20)
	res := f.uploadOK(t, `C:\fakepath\O1234.nc`, []byte(sampleNC))
	if res.VersionNo != 1 || !res.IsCurrent || res.FileName != "O1234.nc" || res.Encoding != "utf-8" {
		t.Fatalf("版本异常: %+v", res.Version)
	}
	if res.Parse == nil || res.Parse.Controller != "FANUC" || len(res.Parse.Tools) != 1 || res.Parse.Tools[0].ToolNo != "T01" {
		t.Fatalf("解析结果异常: %+v", res.Parse)
	}
	var list []domain.Version
	decode(t, f.get(t, f.path+"/versions"), &list)
	if len(list) != 1 || list[0].ID != res.ID {
		t.Fatalf("版本列表异常: %+v", list)
	}
	var prog domain.Program
	decode(t, f.get(t, f.path), &prog)
	if prog.CurrentVersionID == nil || *prog.CurrentVersionID != res.ID {
		t.Fatal("上传后应成为当前版本")
	}
}

func TestUploadValidation(t *testing.T) {
	f := newFixture(t, 64)
	if rec := f.upload(t, "", nil, ""); rec.Code != 400 {
		t.Fatalf("缺少 file 应 400，得到 %d", rec.Code)
	}
	if rec := f.upload(t, "a.nc", []byte{}, ""); rec.Code != 400 {
		t.Fatalf("空文件应 400，得到 %d", rec.Code)
	}
	if rec := f.upload(t, "a.nc", bytes.Repeat([]byte("G01\n"), 100), ""); rec.Code != 413 {
		t.Fatalf("超限文件应 413，得到 %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, f.h, "POST", f.path+"/versions", `{"x":1}`, f.ck); rec.Code != 400 {
		t.Fatalf("非 multipart 应 400，得到 %d", rec.Code)
	}
	f2 := newFixture(t, 1<<20)
	req := httptest.NewRequest("POST", "/api/programs/999/versions", strings.NewReader(""))
	req.Header.Set("Origin", "http://example.com")
	req.AddCookie(f2.ck)
	rec := httptest.NewRecorder()
	f2.h.ServeHTTP(rec, req)
	if rec.Code != 400 && rec.Code != 404 {
		t.Fatalf("不存在的程序应 4xx，得到 %d", rec.Code)
	}
	var list []domain.Version
	decode(t, f.get(t, f.path+"/versions"), &list)
	if len(list) != 0 {
		t.Fatal("失败的上传不应产生版本")
	}
}

func TestUploadToMissingProgramIs404(t *testing.T) {
	f := newFixture(t, 1<<20)
	f.path = "/api/programs/999"
	if rec := f.upload(t, "a.nc", []byte("M30"), ""); rec.Code != 404 {
		t.Fatalf("不存在的程序应 404，得到 %d", rec.Code)
	}
}

func TestGBKUploadContentAndDownloadConversion(t *testing.T) {
	f := newFixture(t, 1<<20)
	gbk := []byte("O1 (")
	gbk = append(gbk, 0xbe, 0xab, 0xb3, 0xb5) // "精车" 的 GBK 字节
	gbk = append(gbk, []byte(")\nM30\n")...)
	v := f.uploadOK(t, "jing.nc", gbk)
	if v.Encoding != "gbk" {
		t.Fatalf("应检测为 gbk，得到 %q", v.Encoding)
	}
	var c domain.VersionContent
	decode(t, f.get(t, fmt.Sprintf("/api/versions/%d/content", v.ID)), &c)
	if c.Encoding != "gbk" || !strings.Contains(c.Content, "精车") || c.LineCount != 2 || c.SizeBytes != len(gbk) || !c.IsCurrent {
		t.Fatalf("内容异常: %+v", c)
	}

	dl := f.get(t, fmt.Sprintf("/api/versions/%d/download", v.ID))
	if dl.Code != 200 || !bytes.Equal(dl.Body.Bytes(), gbk) {
		t.Fatal("默认下载应返回原始字节")
	}
	if cd := dl.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "jing.nc") {
		t.Fatalf("Content-Disposition 异常: %q", cd)
	}
	if dl.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("应设置 nosniff")
	}
	utf := f.get(t, fmt.Sprintf("/api/versions/%d/download?encoding=utf-8", v.ID))
	if !strings.Contains(utf.Body.String(), "精车") {
		t.Fatalf("转 UTF-8 下载失败: %q", utf.Body.String())
	}
	if rec := f.get(t, fmt.Sprintf("/api/versions/%d/download?encoding=latin1", v.ID)); rec.Code != 400 {
		t.Fatalf("不支持的编码应 400，得到 %d", rec.Code)
	}
	if rec := f.get(t, "/api/versions/999/download"); rec.Code != 404 {
		t.Fatalf("不存在应 404，得到 %d", rec.Code)
	}
}

func TestDownloadNonASCIIFileName(t *testing.T) {
	f := newFixture(t, 1<<20)
	v := f.uploadOK(t, "壳体粗车.nc", []byte(sampleNC))
	cd := f.get(t, fmt.Sprintf("/api/versions/%d/download", v.ID)).Header().Get("Content-Disposition")
	if !strings.Contains(cd, "filename*=") {
		t.Fatalf("中文文件名应使用 RFC 2231 编码: %q", cd)
	}
}

func TestSaveContentAsNewVersionAndEncodingWarning(t *testing.T) {
	f := newFixture(t, 1<<20)
	base := f.uploadOK(t, "O1234.nc", []byte{'(', 0xbe, 0xab, 0xb3, 0xb5, ')', '\n', 'M', '3', '0', '\n'}) // GBK

	// 编码留空时沿用当前版本的编码（gbk）；Ø 无法用 GBK 表示，必须给出警告
	body := `{"content":"(精车外圆 Ø60)\nM30\n","changeNote":"改注释"}`
	rec := do(t, f.h, "POST", f.path+"/versions/content", body, f.ck)
	var res domain.VersionSaveResult
	decode(t, rec, &res)
	if rec.Code != 201 || res.VersionNo != 2 || res.Encoding != "gbk" || res.FileName != "O1234.nc" || res.ChangeNote != "改注释" {
		t.Fatalf("新版本异常: %d %+v", rec.Code, res)
	}
	if !strings.Contains(res.Warning, "Ø") {
		t.Fatalf("应警告 Ø 被替换: %q", res.Warning)
	}
	if base.ID == res.ID {
		t.Fatal("应新增版本")
	}

	rec = do(t, f.h, "POST", f.path+"/versions/content", `{"content":"M30\n","encoding":"utf-8"}`, f.ck)
	res = domain.VersionSaveResult{} // 复用变量会残留 omitempty 省略的旧 Warning
	decode(t, rec, &res)
	if res.Encoding != "utf-8" || res.Warning != "" || res.VersionNo != 3 {
		t.Fatalf("显式 utf-8 保存异常: %+v", res)
	}
	if rec := do(t, f.h, "POST", f.path+"/versions/content", `{"content":""}`, f.ck); rec.Code != 400 {
		t.Fatalf("空内容应 400，得到 %d", rec.Code)
	}
	if rec := do(t, f.h, "POST", "/api/programs/999/versions/content", `{"content":"M30"}`, f.ck); rec.Code != 404 {
		t.Fatalf("不存在的程序应 404，得到 %d", rec.Code)
	}
}

func TestSaveContentForProgramWithoutVersionUsesProgramNoAsName(t *testing.T) {
	f := newFixture(t, 1<<20)
	var res domain.VersionSaveResult
	decode(t, do(t, f.h, "POST", f.path+"/versions/content", `{"content":"M30\n"}`, f.ck), &res)
	if res.FileName != "O1234.nc" || res.VersionNo != 1 || res.Encoding != "utf-8" {
		t.Fatalf("首版保存异常: %+v", res)
	}
}

func TestOverwriteContentKeepsVersionNumber(t *testing.T) {
	f := newFixture(t, 1<<20)
	v1 := f.uploadOK(t, "a.nc", []byte("M30\n"))
	f.uploadOK(t, "b.nc", []byte("G00\nM30\n"))

	rec := do(t, f.h, "PUT", fmt.Sprintf("/api/versions/%d/content", v1.ID), `{"content":"G01 X1\nM30\n"}`, f.ck)
	var res domain.VersionSaveResult
	decode(t, rec, &res)
	if rec.Code != 200 || res.ID != v1.ID || res.VersionNo != 1 || res.SHA256 == v1.SHA256 {
		t.Fatalf("覆盖异常: %d %+v", rec.Code, res)
	}
	var c domain.VersionContent
	decode(t, f.get(t, fmt.Sprintf("/api/versions/%d/content", v1.ID)), &c)
	if c.Content != "G01 X1\nM30\n" {
		t.Fatalf("覆盖后内容不对: %q", c.Content)
	}
	var list []domain.Version
	decode(t, f.get(t, f.path+"/versions"), &list)
	if len(list) != 2 {
		t.Fatal("覆盖不应新增版本")
	}
	if rec := do(t, f.h, "PUT", "/api/versions/999/content", `{"content":"M30"}`, f.ck); rec.Code != 404 {
		t.Fatalf("不存在的版本应 404，得到 %d", rec.Code)
	}
}

func TestSetCurrentVersionRollback(t *testing.T) {
	f := newFixture(t, 1<<20)
	v1 := f.uploadOK(t, "a.nc", []byte("M30\n"))
	f.uploadOK(t, "b.nc", []byte("G00\nM30\n"))

	var prog domain.Program
	rec := do(t, f.h, "PUT", f.path+"/current-version", fmt.Sprintf(`{"versionId":%d}`, v1.ID), f.ck)
	decode(t, rec, &prog)
	if rec.Code != 200 || *prog.CurrentVersionNo != 1 {
		t.Fatalf("回滚失败: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, f.h, "PUT", f.path+"/current-version", `{"versionId":999}`, f.ck); rec.Code != 404 {
		t.Fatalf("不存在的版本应 404，得到 %d", rec.Code)
	}
}

func TestDiffEndpoint(t *testing.T) {
	f := newFixture(t, 1<<20)
	v1 := f.uploadOK(t, "a.nc", []byte("O1\nG00 X0\nM30\n"))
	v2 := f.uploadOK(t, "a.nc", []byte("O1\nG01 X0 F100\nM30\nM99\n"))

	var d domain.DiffResult
	rec := f.get(t, fmt.Sprintf("/api/versions/%d/diff?with=%d", v1.ID, v2.ID))
	decode(t, rec, &d)
	if rec.Code != 200 || d.Identical || d.LeftVersionNo != 1 || d.RightVersionNo != 2 {
		t.Fatalf("diff 异常: %d %+v", rec.Code, d)
	}
	types := []string{}
	for _, l := range d.Lines {
		types = append(types, l.Type)
	}
	if strings.Join(types, ",") != "same,change,same,add" {
		t.Fatalf("差异类型 = %v", types)
	}

	decode(t, f.get(t, fmt.Sprintf("/api/versions/%d/diff?with=%d", v1.ID, v1.ID)), &d)
	if !d.Identical {
		t.Fatal("同一版本应 identical")
	}
	if rec := f.get(t, fmt.Sprintf("/api/versions/%d/diff", v1.ID)); rec.Code != 400 {
		t.Fatalf("缺少 with 应 400，得到 %d", rec.Code)
	}
	if rec := f.get(t, fmt.Sprintf("/api/versions/%d/diff?with=999", v1.ID)); rec.Code != 404 {
		t.Fatalf("不存在应 404，得到 %d", rec.Code)
	}
}

func TestDiffRejectsDifferentPrograms(t *testing.T) {
	f := newFixture(t, 1<<20)
	v1 := f.uploadOK(t, "a.nc", []byte("M30\n"))
	var op domain.Operation
	decode(t, do(t, f.h, "POST", "/api/drawings/1/operations", `{"opNo":20}`, f.ck), &op)
	var other domain.Program
	decode(t, do(t, f.h, "POST", fmt.Sprintf("/api/operations/%d/programs", op.ID), `{"programNo":"O2"}`, f.ck), &other)
	f2 := &fixture{h: f.h, ck: f.ck, path: fmt.Sprintf("/api/programs/%d", other.ID)}
	v2 := f2.uploadOK(t, "b.nc", []byte("G00\n"))
	if rec := f.get(t, fmt.Sprintf("/api/versions/%d/diff?with=%d", v1.ID, v2.ID)); rec.Code != 400 {
		t.Fatalf("跨程序比较应 400，得到 %d", rec.Code)
	}
}

func TestParseEndpoint(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	body := fmt.Sprintf(`{"content":%q}`, "%\nO1\nT0202\nM30\n")
	var res domain.ParseResult
	rec := do(t, h, "POST", "/api/nc/parse", body, ck)
	decode(t, rec, &res)
	if rec.Code != 200 || len(res.Tools) != 1 || res.Tools[0].ToolNo != "T02" || res.Tools[0].OffsetNo != "D02" {
		t.Fatalf("解析异常: %d %+v", rec.Code, res)
	}
	decode(t, do(t, h, "POST", "/api/nc/parse", `{"content":""}`, ck), &res)
	if len(res.Warnings) == 0 || res.Tools == nil {
		t.Fatal("空内容应返回警告且 tools 为空数组")
	}
}

func TestIdenticalUploadSharesStorage(t *testing.T) {
	f := newFixture(t, 1<<20)
	a := f.uploadOK(t, "first.nc", []byte(sampleNC))
	b := f.uploadOK(t, "second.nc", []byte(sampleNC))
	if a.SHA256 != b.SHA256 || b.VersionNo != 2 || b.FileName != "second.nc" || a.FileName != "first.nc" {
		t.Fatalf("相同内容应共享存储、各自保留文件名: %+v %+v", a.Version, b.Version)
	}
}
