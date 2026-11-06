package meters

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st := store.New()
	st.Load(nil,
		[]store.Band{{Code: "A", Rate: 20950}, {Code: "B", Rate: 6330}},
		[]store.Flat{{ID: 1, Block: "A", Number: 1, MeterID: 10}},
		[]store.Meter{{ID: 10, FlatID: 1, Serial: "0471-0001-0001", Band: "A", Balance: energy.KWh(1000)}},
	)
	return NewService(st, NewDemoVendor(0)), st
}

func TestNewQuote(t *testing.T) {
	q := NewQuote(money.Naira(10_000), 20950)
	if q.Net != 930233 || q.VAT != 69767 {
		t.Fatalf("net/vat = %v/%v", q.Net, q.VAT)
	}
	if q.Units != 4440 {
		t.Fatalf("units = %v, want 44.40", q.Units)
	}
}

func TestBuyCreditsMeter(t *testing.T) {
	svc, st := newTestService(t)
	p, err := svc.Buy(context.Background(), BuyRequest{MeterID: 10, Amount: money.Naira(10_000), RequestID: "r1", BoughtBy: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Ref == "" || len(p.Token) != 24 {
		t.Fatalf("unexpected purchase %+v", p)
	}
	m, _ := st.Meter(10)
	if m.Balance != energy.KWh(1000)+p.Units {
		t.Fatalf("balance = %v", m.Balance)
	}
}

func TestBuyRepeatedRequestReturnsFirstPurchase(t *testing.T) {
	svc, st := newTestService(t)
	first, err := svc.Buy(context.Background(), BuyRequest{MeterID: 10, Amount: money.Naira(5_000), RequestID: "same"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Buy(context.Background(), BuyRequest{MeterID: 10, Amount: money.Naira(5_000), RequestID: "same"})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || len(st.Purchases()) != 1 {
		t.Fatalf("resubmitted form created a second purchase")
	}
}

func TestBuyConcurrentSameRequestCreatesOnePurchase(t *testing.T) {
	st := store.New()
	st.Load(nil,
		[]store.Band{{Code: "A", Rate: 20950}, {Code: "B", Rate: 6330}},
		[]store.Flat{{ID: 1, Block: "A", Number: 1, MeterID: 10}},
		[]store.Meter{{ID: 10, FlatID: 1, Serial: "0471-0001-0001", Band: "A", Balance: energy.KWh(1000)}},
	)
	svc := NewService(st, NewDemoVendor(50*time.Millisecond))
	const attempts = 6
	var wg sync.WaitGroup
	start := make(chan struct{})
	purchases := make([]store.Purchase, attempts)
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			purchases[i], errs[i] = svc.Buy(context.Background(), BuyRequest{
				MeterID: 10, Amount: money.Naira(10_000), RequestID: "concurrent", BoughtBy: "test",
			})
		}(i)
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if stored := st.Purchases(); len(stored) != 1 {
		t.Fatalf("stored %d purchases, want 1", len(stored))
	}
	for i := 1; i < attempts; i++ {
		if purchases[i].ID != purchases[0].ID {
			t.Fatalf("purchase %d has ID %d, want %d", i, purchases[i].ID, purchases[0].ID)
		}
	}
	m, _ := st.Meter(10)
	if m.Balance != energy.KWh(1000)+purchases[0].Units {
		t.Fatalf("balance = %v, want one credit of %v", m.Balance, purchases[0].Units)
	}
}

func TestBuyLimits(t *testing.T) {
	svc, _ := newTestService(t)
	for _, amount := range []money.Kobo{money.Naira(999), money.Naira(500_001), 0} {
		if _, err := svc.Buy(context.Background(), BuyRequest{MeterID: 10, Amount: amount}); err != ErrAmount {
			t.Errorf("amount %v: err = %v", amount, err)
		}
	}
}

func TestHistoryNewestFirst(t *testing.T) {
	svc, _ := newTestService(t)
	for _, id := range []string{"a", "b", "c"} {
		if _, err := svc.Buy(context.Background(), BuyRequest{MeterID: 10, Amount: money.Naira(2_000), RequestID: id}); err != nil {
			t.Fatal(err)
		}
	}
	h := svc.History(10)
	if len(h) != 3 || h[0].RequestID != "c" || h[2].RequestID != "a" {
		t.Fatalf("history order: %v %v %v", h[0].RequestID, h[1].RequestID, h[2].RequestID)
	}
}
