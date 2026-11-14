package httpapi

import (
	"fmt"
	"net/http"
	"testing"

	"cnc-manager/internal/domain"
)

func TestDrawingPriceCreateEditClearAndLegacyClients(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	var d domain.Drawing
	created := do(t, h, "POST", "/api/drawings", `{"drawingNo":"PRICE","price":"0.29"}`, ck)
	if created.Code != http.StatusCreated {
		t.Fatal(created.Code, created.Body.String())
	}
	decode(t, created, &d)
	assertPrice := func(got *string, want string) {
		t.Helper()
		if want == "" {
			if got != nil {
				t.Fatal("expected empty", *got)
			}
		} else if got == nil || *got != want {
			t.Fatalf("price: %v want %s", got, want)
		}
	}
	assertPrice(d.Price, "0.29")
	base := fmt.Sprintf("/api/drawings/%d", d.ID)
	for _, tc := range []struct{ body, want string }{
		{`{"drawingNo":"PRICE","price":"1234.50"}`, "1234.50"},
		{`{"drawingNo":"PRICE","material":"45#"}`, "1234.50"},
		{`{"drawingNo":"PRICE","price":0}`, "0.00"},
		{`{"drawingNo":"PRICE","price":""}`, ""},
		{`{"drawingNo":"PRICE","price":88.8}`, "88.80"},
		{`{"drawingNo":"PRICE","price":null}`, ""},
	} {
		rec := do(t, h, "PUT", base, tc.body, ck)
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		decode(t, rec, &d)
		assertPrice(d.Price, tc.want)
		var detail domain.DrawingDetail
		decode(t, do(t, h, "GET", base+"/detail", "", ck), &detail)
		assertPrice(detail.Price, tc.want)
		var page domain.Page[domain.Drawing]
		decode(t, do(t, h, "GET", "/api/drawings?q=PRICE", "", ck), &page)
		if len(page.Items) != 1 {
			t.Fatal(page)
		}
		assertPrice(page.Items[0].Price, tc.want)
	}
	for _, value := range []string{`"-1"`, `"1.234"`, `"1000000000"`, `true`} {
		rec := do(t, h, "PUT", base, `{"drawingNo":"SHOULD-NOT-SAVE","price":`+value+`}`, ck)
		if rec.Code != 400 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	var unchanged domain.DrawingDetail
	decode(t, do(t, h, "GET", base+"/detail", "", ck), &unchanged)
	if unchanged.DrawingNo != "PRICE" || unchanged.Price != nil {
		t.Fatal("invalid edit changed drawing")
	}
}
