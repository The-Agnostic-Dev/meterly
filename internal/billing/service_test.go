package billing

import (
	"testing"
	"time"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

func TestStatus(t *testing.T) {
	due := time.Date(2026, 10, 10, 0, 0, 0, 0, clock.Lagos)
	inv := store.Invoice{Amount: money.Naira(35_000), DueOn: due}
	if got := Status(inv, due); got != StatusUnpaid {
		t.Errorf("on due date: %s", got)
	}
	if got := Status(inv, due.AddDate(0, 0, 1)); got != StatusOverdue {
		t.Errorf("after due date: %s", got)
	}
	inv.Payments = []store.Payment{{Amount: money.Naira(10_000)}}
	if got := Status(inv, due); got != StatusPartPaid {
		t.Errorf("part paid: %s", got)
	}
	inv.Payments = append(inv.Payments, store.Payment{Amount: money.Naira(25_000)})
	if got := Status(inv, due.AddDate(0, 1, 0)); got != StatusPaid {
		t.Errorf("paid: %s", got)
	}
}

func TestRecordPayment(t *testing.T) {
	st := store.New()
	st.Load(nil, nil, []store.Flat{{ID: 1, Block: "B", Number: 4, Type: "2-bed"}}, nil)
	inv := st.AddInvoice(store.Invoice{FlatID: 1, Month: clock.MonthKey(clock.Now()), Amount: money.Naira(35_000),
		IssuedOn: clock.StartOfMonth(clock.Now()), DueOn: clock.StartOfMonth(clock.Now()).AddDate(0, 0, 9)})
	svc := NewService(st)

	if _, err := svc.RecordPayment(inv.ID, money.Naira(40_000), clock.Today(), "acc"); err == nil {
		t.Fatal("overpayment should be refused")
	}
	if _, err := svc.RecordPayment(inv.ID, 0, clock.Today(), "acc"); err != ErrPaymentAmount {
		t.Fatalf("zero payment: %v", err)
	}
	got, err := svc.RecordPayment(inv.ID, money.Naira(20_000), clock.Today(), "acc")
	if err != nil || got.Balance() != money.Naira(15_000) {
		t.Fatalf("balance %v, err %v", got.Balance(), err)
	}
	if lines := svc.Lines("", "outstanding"); len(lines) != 1 {
		t.Fatalf("outstanding lines = %d", len(lines))
	}
}
