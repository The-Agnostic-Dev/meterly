package web

import (
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/meters"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/seed"
	"github.com/palmgrove-estates/meterly/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts, _ := newTestServerWithStore(t, 0)
	return ts
}

func newTestServerWithStore(t *testing.T, delay time.Duration) (*httptest.Server, *store.Store) {
	t.Helper()
	st := store.New()
	seed.Load(st)
	srv, err := New(Deps{Store: st, Vendor: meters.NewDemoVendor(delay)})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, st
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

func TestConcurrentBuySameRequestMakesOneReceipt(t *testing.T) {
	ts, st := newTestServerWithStore(t, 50*time.Millisecond)
	c := client(t, ts, "chioma.okafor@gmail.com")
	const attempts = 5
	start := make(chan struct{})
	paths := make([]string, attempts)
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			res, err := c.PostForm(ts.URL+"/meters/502/buy", url.Values{"amount": {"10,000"}, "request_id": {"race-buy"}})
			if err != nil {
				errs[i] = err
				return
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			paths[i] = res.Request.URL.Path
		}(i)
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var stored int
	for _, p := range st.Purchases() {
		if p.RequestID == "race-buy" {
			stored++
		}
	}
	if stored != 1 {
		t.Fatalf("stored %d purchases for one request, want 1", stored)
	}
	if !strings.HasPrefix(paths[0], "/purchases/") {
		t.Fatalf("receipt path = %q", paths[0])
	}
	for _, path := range paths[1:] {
		if path != paths[0] {
			t.Fatalf("receipts differ: %v", paths)
		}
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

func TestTokenHistoryListsEveryTokenAndReceipt(t *testing.T) {
	ts, st := newTestServerWithStore(t, 0)
	c := client(t, ts, "fm@palmgrovecourt.ng")

	countPattern := regexp.MustCompile(`([0-9]+) tokens`)
	rowPattern := regexp.MustCompile(`data-ref="([^"]+)"`)
	nextTagPattern := regexp.MustCompile(`(?i)<a[^>]*rel="next"[^>]*>`)
	hrefPattern := regexp.MustCompile(`(?i)href="([^"]*)"`)

	var refs []string
	headerCount := -1
	pageURL := ts.URL + "/tokens"
	for pageURL != "" {
		current := pageURL
		code, body := get(t, c, current)
		if code != http.StatusOK {
			t.Fatalf("%s: status %d", current, code)
		}
		if headerCount < 0 {
			m := countPattern.FindStringSubmatch(body)
			if m == nil {
				t.Fatal("token history does not show a token count")
			}
			n, err := strconv.Atoi(m[1])
			if err != nil {
				t.Fatal(err)
			}
			headerCount = n
		}
		for _, m := range rowPattern.FindAllStringSubmatch(body, -1) {
			refs = append(refs, m[1])
		}
		pageURL = ""
		if tag := nextTagPattern.FindString(body); tag != "" {
			m := hrefPattern.FindStringSubmatch(tag)
			if m == nil {
				t.Fatal("next link has no href")
			}
			base, err := url.Parse(current)
			if err != nil {
				t.Fatal(err)
			}
			href, err := url.Parse(strings.ReplaceAll(m[1], "&amp;", "&"))
			if err != nil {
				t.Fatal(err)
			}
			pageURL = base.ResolveReference(href).String()
		}
	}

	purchases := st.Purchases()
	if headerCount != len(refs) {
		t.Fatalf("header says %d tokens, gathered %d rows", headerCount, len(refs))
	}
	if headerCount != len(purchases) {
		t.Fatalf("header says %d tokens, store has %d purchases", headerCount, len(purchases))
	}
	seen := map[string]int{}
	for _, ref := range refs {
		seen[ref]++
	}
	for _, p := range purchases {
		if seen[p.Ref] != 1 {
			t.Errorf("purchase %s appears %d times in token history, want 1", p.Ref, seen[p.Ref])
		}
	}
	if len(seen) != len(purchases) {
		t.Fatalf("token history has %d distinct refs, want %d", len(seen), len(purchases))
	}
	for _, p := range purchases {
		code, _ := get(t, c, ts.URL+"/purchases/"+strconv.Itoa(p.ID))
		if code != http.StatusOK {
			t.Errorf("receipt for purchase %d: status %d", p.ID, code)
		}
	}
}

func buyUnits(t *testing.T, c *http.Client, ts *httptest.Server, meterID int, amount, requestID string) {
	t.Helper()
	res, err := c.PostForm(ts.URL+"/meters/"+strconv.Itoa(meterID)+"/buy", url.Values{
		"amount":     {amount},
		"request_id": {requestID},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		t.Fatalf("buy %s: status %d: %s", requestID, res.StatusCode, body)
	}
}

func resolvePurchase(t *testing.T, c *http.Client, ts *httptest.Server, purchaseID int, action string) {
	t.Helper()
	res, err := c.PostForm(ts.URL+"/purchases/"+strconv.Itoa(purchaseID)+"/"+action, url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode >= http.StatusInternalServerError {
		t.Fatalf("%s purchase %d: status %d: %s", action, purchaseID, res.StatusCode, body)
	}
}

func paidInMonth(st *store.Store, flatID int, month string) money.Kobo {
	var total money.Kobo
	for _, inv := range st.Invoices() {
		if inv.FlatID == flatID && inv.Month == month {
			total += inv.Paid()
		}
	}
	return total
}

func TestRefundPurchaseRestoresBalanceOnce(t *testing.T) {
	ts, st := newTestServerWithStore(t, 0)
	resident := client(t, ts, "chioma.okafor@gmail.com")
	manager := client(t, ts, "fm@palmgrovecourt.ng")

	before, err := st.Meter(502)
	if err != nil {
		t.Fatal(err)
	}
	const requestID = "refund-flow"
	buyUnits(t, resident, ts, 502, "10,000", requestID)

	p, ok := st.PurchaseByRequest(requestID)
	if !ok {
		t.Fatalf("purchase %q was not stored", requestID)
	}
	afterBuy, err := st.Meter(502)
	if err != nil {
		t.Fatal(err)
	}
	if afterBuy.Balance != before.Balance+p.Units {
		t.Fatalf("balance after buy = %d, want %d", afterBuy.Balance, before.Balance+p.Units)
	}

	resolvePurchase(t, manager, ts, p.ID, "refund")

	p, err = st.Purchase(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Refunded {
		t.Fatalf("purchase %d was not marked refunded: %+v", p.ID, p)
	}
	if p.ResolvedBy != "Tunde Bakare" {
		t.Fatalf("refund recorded by %q, want Tunde Bakare", p.ResolvedBy)
	}
	afterRefund, err := st.Meter(502)
	if err != nil {
		t.Fatal(err)
	}
	if afterRefund.Balance != before.Balance {
		t.Fatalf("balance after refund = %d, want %d", afterRefund.Balance, before.Balance)
	}

	resolvePurchase(t, manager, ts, p.ID, "refund")
	afterDuplicate, err := st.Meter(502)
	if err != nil {
		t.Fatal(err)
	}
	if afterDuplicate.Balance != before.Balance {
		t.Fatalf("duplicate refund changed balance to %d, want %d", afterDuplicate.Balance, before.Balance)
	}

	if _, err := st.RefundPurchase(p.ID, "Tunde Bakare", clock.Now()); !errors.Is(err, store.ErrAlreadyResolved) {
		t.Fatalf("second store refund error = %v, want %v", err, store.ErrAlreadyResolved)
	}
}

func TestCreditPurchaseAppliesNextMonthPaymentOnce(t *testing.T) {
	ts, st := newTestServerWithStore(t, 0)
	resident := client(t, ts, "chioma.okafor@gmail.com")
	manager := client(t, ts, "fm@palmgrovecourt.ng")

	before, err := st.Meter(502)
	if err != nil {
		t.Fatal(err)
	}
	const requestID = "credit-flow"
	buyUnits(t, resident, ts, 502, "10,000", requestID)

	p, ok := st.PurchaseByRequest(requestID)
	if !ok {
		t.Fatalf("purchase %q was not stored", requestID)
	}

	nextMonth := clock.StartOfMonth(clock.Today()).AddDate(0, 1, 0)
	monthKey := clock.MonthKey(nextMonth)
	beforePaid := paidInMonth(st, 102, monthKey)

	resolvePurchase(t, manager, ts, p.ID, "credit")

	p, err = st.Purchase(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Credited {
		t.Fatalf("purchase %d was not marked credited: %+v", p.ID, p)
	}
	if p.ResolvedBy != "Tunde Bakare" {
		t.Fatalf("credit recorded by %q, want Tunde Bakare", p.ResolvedBy)
	}
	afterCredit, err := st.Meter(502)
	if err != nil {
		t.Fatal(err)
	}
	if afterCredit.Balance != before.Balance {
		t.Fatalf("balance after credit = %d, want %d", afterCredit.Balance, before.Balance)
	}

	if p.CreditInvoiceID == 0 {
		t.Fatalf("purchase %d has no credit invoice", p.ID)
	}
	inv, err := st.Invoice(p.CreditInvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.FlatID != 102 {
		t.Fatalf("credit invoice flat = %d, want 102", inv.FlatID)
	}
	if inv.Month != monthKey {
		t.Fatalf("credit invoice month = %q, want %q", inv.Month, monthKey)
	}
	afterPaid := paidInMonth(st, 102, monthKey)
	if afterPaid != beforePaid+p.Amount {
		t.Fatalf("paid in %s = %d, want %d", monthKey, afterPaid, beforePaid+p.Amount)
	}

	resolvePurchase(t, manager, ts, p.ID, "credit")
	afterDuplicate, err := st.Meter(502)
	if err != nil {
		t.Fatal(err)
	}
	if afterDuplicate.Balance != before.Balance {
		t.Fatalf("duplicate credit changed balance to %d, want %d", afterDuplicate.Balance, before.Balance)
	}
	if got := paidInMonth(st, 102, monthKey); got != afterPaid {
		t.Fatalf("duplicate credit changed payments in %s to %d, want %d", monthKey, got, afterPaid)
	}

	if _, _, err := st.CreditPurchase(p.ID, monthKey, p.Amount, clock.Today(), clock.Today(), "Tunde Bakare", clock.Now()); !errors.Is(err, store.ErrAlreadyResolved) {
		t.Fatalf("second store credit error = %v, want %v", err, store.ErrAlreadyResolved)
	}
}
