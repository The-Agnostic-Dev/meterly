package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/palmgrove-estates/meterly/internal/meters"
	"github.com/palmgrove-estates/meterly/internal/seed"
	"github.com/palmgrove-estates/meterly/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	st := store.New()
	seed.Load(st)
	srv, err := New(Deps{Store: st, Vendor: meters.NewDemoVendor(0)})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func client(t *testing.T, ts *httptest.Server, email string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	res, err := c.PostForm(ts.URL+"/login", url.Values{"email": {email}, "password": {seed.DemoPassword}})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return c
}

func get(t *testing.T, c *http.Client, u string) (int, string) {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestPagesRenderForManager(t *testing.T) {
	ts := newTestServer(t)
	c := client(t, ts, "fm@palmgrovecourt.ng")
	for _, path := range []string{"/", "/flats", "/flats/101", "/meters/501", "/tokens", "/charges", "/reports/usage"} {
		if code, body := get(t, c, ts.URL+path); code != http.StatusOK || !strings.Contains(body, "Meterly") {
			t.Errorf("%s: status %d", path, code)
		}
	}
}

func TestLoginRequired(t *testing.T) {
	ts := newTestServer(t)
	res, err := http.Get(ts.URL + "/flats")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.Request.URL.Path != "/login" {
		t.Fatalf("ended on %s", res.Request.URL.Path)
	}
}

func TestResidentSeesOnlyOwnFlat(t *testing.T) {
	ts := newTestServer(t)
	c := client(t, ts, "ibrahim.musa@gmail.com")
	if code, _ := get(t, c, ts.URL+"/flats/110"); code != http.StatusOK {
		t.Fatalf("own flat: %d", code)
	}
	if code, _ := get(t, c, ts.URL+"/flats/101"); code != http.StatusNotFound {
		t.Fatalf("other flat: %d", code)
	}
	if code, _ := get(t, c, ts.URL+"/tokens"); code != http.StatusForbidden {
		t.Fatalf("token history: %d", code)
	}
}

func TestBuyUnitsShowsReceipt(t *testing.T) {
	ts := newTestServer(t)
	c := client(t, ts, "chioma.okafor@gmail.com")
	res, err := c.PostForm(ts.URL+"/meters/502/buy", url.Values{"amount": {"10,000"}, "request_id": {"t1"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.HasPrefix(res.Request.URL.Path, "/purchases/") || !strings.Contains(string(body), "44.40 kWh") {
		t.Fatalf("ended on %s", res.Request.URL.Path)
	}
}

func TestReceiptFiguresAddUp(t *testing.T) {
	ts := newTestServer(t)
	c := client(t, ts, "chioma.okafor@gmail.com")
	res, err := c.PostForm(ts.URL+"/meters/502/buy", url.Values{"amount": {"50,000"}, "request_id": {"t1"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	receipt := string(body)
	if !strings.HasPrefix(res.Request.URL.Path, "/purchases/") {
		t.Fatalf("ended on %s", res.Request.URL.Path)
	}
	for _, want := range []string{"₦50,000.00", "₦3,488.37", "₦46,511.63", "222.01 kWh"} {
		if !strings.Contains(receipt, want) {
			t.Errorf("receipt does not show %s", want)
		}
	}
}

func TestResidentDetailsSaved(t *testing.T) {
	ts := newTestServer(t)
	c := client(t, ts, "fm@palmgrovecourt.ng")
	res, err := c.PostForm(ts.URL+"/flats/105", url.Values{"name": {"Yusuf  Abdullahi"}, "phone": {"0812 778 3019"}, "email": {"Yusuf@Example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), "Resident details saved") || !strings.Contains(string(body), "08127783019") {
		t.Fatal("details were not saved")
	}
}

func TestReceiptPrintButton(t *testing.T) {
	ts := newTestServer(t)
	c := client(t, ts, "chioma.okafor@gmail.com")
	res, err := c.PostForm(ts.URL+"/meters/502/buy", url.Values{"amount": {"10,000"}, "request_id": {"t1"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	receipt := string(body)
	if !strings.HasPrefix(res.Request.URL.Path, "/purchases/") {
		t.Fatalf("ended on %s", res.Request.URL.Path)
	}
	if !strings.Contains(receipt, `data-action="print"`) || !strings.Contains(receipt, `type="button"`) {
		t.Fatal("receipt has no print button")
	}
	code, js := get(t, c, ts.URL+"/static/app.js")
	if code != http.StatusOK {
		t.Fatalf("app.js: %d", code)
	}
	if !strings.Contains(js, "function printPage(") || !strings.Contains(js, "window.print()") {
		t.Fatal("app.js has no named printPage handler calling window.print()")
	}
	if !strings.Contains(js, `[data-action="print"]`) || !strings.Contains(js, "printPage") {
		t.Fatal("app.js does not register printPage for the print button")
	}
}

func TestPagerPositions(t *testing.T) {
	p := NewPager(25, 2, 10)
	if p.From() != 11 || p.To() != 20 || p.Pages() != 3 || !p.HasNext() || !p.HasPrev() {
		t.Fatalf("page 2: %+v from %d to %d", p, p.From(), p.To())
	}
	last := NewPager(25, 9, 10)
	if last.Page != 3 || last.To() != 25 {
		t.Fatalf("clamped: %+v", last)
	}
}
