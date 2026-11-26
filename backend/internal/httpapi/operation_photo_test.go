package httpapi

import (
	"bytes"
	"cnc-manager/internal/backup"
	"image"
	"image/jpeg"
	"testing"
)

func TestOperationPhotoRoundTrip(t *testing.T) {
	h, _ := backupServer(t)
	admin := setupAdmin(t, h)
	const path = "/api/operations/1/photos"
	if rec := do(t, h, "POST", "/api/drawings", `{"drawingNo":"PHOTO"}`, admin); rec.Code != 201 {
		t.Fatal(rec.Body)
	}
	if rec := do(t, h, "POST", "/api/drawings/1/operations", `{"opNo":10}`, admin); rec.Code != 201 {
		t.Fatal(rec.Body)
	}
	if rec := do(t, h, "POST", path, "not an image", admin); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	var digest string
	for range 2 {
		rec := do(t, h, "POST", path, data.String(), admin)
		if rec.Code != 201 {
			t.Fatal(rec.Body)
		}
		decode(t, rec, &digest)
	}
	var photos []string
	decode(t, do(t, h, "GET", path, "", admin), &photos)
	if len(photos) != 1 || photos[0] != digest {
		t.Fatal(photos)
	}
	raw := do(t, h, "GET", "/api/backup/export", "", admin)
	if raw.Code != 200 {
		t.Fatal(raw.Body)
	}
	if rec := do(t, h, "DELETE", path+"/"+digest, "", admin); rec.Code != 204 {
		t.Fatal(rec.Body)
	}
	if rec := do(t, h, "GET", path+"/"+digest, "", admin); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	var preview backup.Preview
	decode(t, uploadBackup(t, h, "/api/backup/inspect", raw.Body.Bytes(), "", "", admin), &preview)
	rec := uploadBackup(t, h, "/api/backup/import", raw.Body.Bytes(), preview.SHA256, "覆盖导入", admin)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	admin = sessionCookie(t, do(t, h, "POST", "/api/auth/login", `{"username":"admin","password":"password123"}`))
	restored := do(t, h, "GET", path+"/"+digest, "", admin)
	if restored.Code != 200 || !bytes.Equal(restored.Body.Bytes(), data.Bytes()) || restored.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatal("photo restore failed", restored.Code)
	}
	if rec := do(t, h, "DELETE", "/api/operations/1", "", admin); rec.Code != 204 {
		t.Fatal(rec.Body)
	}
}
