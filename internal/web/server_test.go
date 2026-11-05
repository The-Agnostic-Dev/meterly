package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/palmgrove-estates/meterly/internal/clock"
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

func TestRecordOverpaymentPersistsAndCarriesCredit(t *testing.T) {
	st := store.New()
	seed.Load(st)
	srv, err := New(Deps{Store: st, Vendor: meters.NewDemoVendor(0)})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	c := client(t, ts, "accounts@palmgrovecourt.ng")

	thisMonth := clock.MonthKey(clock.Now())
	var inv store.Invoice
	for _, candidate := range st.Invoices() {
		if candidate.FlatID == 114 && candidate.Month == thisMonth {
			inv = candidate
		}
	}
	if inv.ID == 0 {
		t.Fatal("no current-month invoice for C2")
	}

	res, err := c.PostForm(ts.URL+"/charges/"+itoa(inv.ID)+"/payments", url.Values{"amount": {"40,000"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), `"ok":true`) || !strings.Contains(string(body), `"balance":"₦0.00"`) {
		t.Fatalf("payment response: %s", body)
	}

	code, page := get(t, c, ts.URL+"/charges/"+itoa(inv.ID))
	if code != http.StatusOK {
		t.Fatalf("invoice page after refresh: status %d", code)
	}
	if !strings.Contains(page, `<td class="num">₦25,000.00</td>`) {
		t.Fatal("current invoice does not show the ₦25,000.00 payment row")
	}
	if !strings.Contains(page, `id="balance">₦0.00</strong>`) {
		t.Fatal("current invoice does not show a ₦0.00 balance")
	}

	nextMonth := clock.MonthKey(clock.StartOfMonth(clock.Now()).AddDate(0, 1, 0))
	var nextInv store.Invoice
	for _, candidate := range st.Invoices() {
		if candidate.FlatID == 114 && candidate.Month == nextMonth {
			nextInv = candidate
		}
	}
	if nextInv.ID == 0 {
		t.Fatal("no next-month invoice for C2 after payment")
	}
	code, nextPage := get(t, c, ts.URL+"/charges/"+itoa(nextInv.ID))
	if code != http.StatusOK {
		t.Fatalf("next invoice page: status %d", code)
	}
	if !strings.Contains(nextPage, `<td class="num">₦15,000.00</td>`) {
		t.Fatal("next invoice does not show the ₦15,000.00 credit payment row")
	}
	if !strings.Contains(nextPage, `id="balance">₦10,000.00</strong>`) {
		t.Fatal("next invoice does not show a ₦10,000.00 balance")
	}
}

func TestPaymentsJSOnlyShowsSuccessOnOK(t *testing.T) {
	ts := newTestServer(t)
	c := client(t, ts, "accounts@palmgrovecourt.ng")
	code, js := get(t, c, ts.URL+"/static/payments.js")
	if code != http.StatusOK {
		t.Fatalf("payments.js: %d", code)
	}
	okBranch := strings.Index(js, "data.ok")
	success := strings.Index(js, "Payment recorded")
	if okBranch == -1 || success == -1 || okBranch > success {
		t.Fatal("payments.js must branch on data.ok before showing 'Payment recorded'")
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
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
