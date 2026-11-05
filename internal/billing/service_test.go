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

	if _, err := svc.RecordPayment(inv.ID, 0, clock.Today(), "acc"); err != ErrPaymentAmount {
		t.Fatalf("zero payment: %v", err)
	}
	got, err := svc.RecordPayment(inv.ID, money.Naira(40_000), clock.Today(), "acc")
	if err != nil {
		t.Fatalf("overpayment should be accepted: %v", err)
	}
	if got.Balance() != 0 {
		t.Fatalf("balance after overpayment = %v", got.Balance())
	}
	if len(got.Payments) != 1 || got.Payments[0].Amount != money.Naira(35_000) {
		t.Fatalf("invoice payments = %+v", got.Payments)
	}
	nextMonth := clock.MonthKey(clock.Now().AddDate(0, 1, 0))
	var credit *store.Invoice
	for _, candidate := range st.Invoices() {
		if candidate.FlatID == 1 && candidate.Month == nextMonth {
			c := candidate
			credit = &c
		}
	}
	if credit == nil {
		t.Fatal("no invoice created for next month")
	}
	if len(credit.Payments) != 1 || credit.Payments[0].Amount != money.Naira(5_000) {
		t.Fatalf("next month credit = %+v", credit.Payments)
	}

	inv2 := st.AddInvoice(store.Invoice{FlatID: 1, Month: clock.MonthKey(clock.Now()), Amount: money.Naira(35_000),
		IssuedOn: clock.StartOfMonth(clock.Now()), DueOn: clock.StartOfMonth(clock.Now()).AddDate(0, 0, 9)})
	got2, err := svc.RecordPayment(inv2.ID, money.Naira(20_000), clock.Today(), "acc")
	if err != nil || got2.Balance() != money.Naira(15_000) {
		t.Fatalf("balance %v, err %v", got2.Balance(), err)
	}
	if lines := svc.Lines("", "outstanding"); len(lines) != 2 {
		t.Fatalf("outstanding lines = %d", len(lines))
	}
}

func TestRecordPaymentOverpaymentCreditsExistingNextInvoice(t *testing.T) {
	st := store.New()
	st.Load(nil, nil, []store.Flat{{ID: 1, Block: "B", Number: 4, Type: "2-bed"}}, nil)
	start := clock.StartOfMonth(clock.Now())
	inv := st.AddInvoice(store.Invoice{FlatID: 1, Month: clock.MonthKey(clock.Now()), Amount: money.Naira(35_000),
		IssuedOn: start, DueOn: start.AddDate(0, 0, 9)})
	next := start.AddDate(0, 1, 0)
	existing := st.AddInvoice(store.Invoice{FlatID: 1, Month: clock.MonthKey(next), Amount: money.Naira(35_000),
		IssuedOn: next, DueOn: next.AddDate(0, 0, 9)})
	svc := NewService(st)

	got, err := svc.RecordPayment(inv.ID, money.Naira(40_000), clock.Today(), "acc")
	if err != nil || got.Balance() != 0 {
		t.Fatalf("balance %v, err %v", got.Balance(), err)
	}
	updated, err := st.Invoice(existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Payments) != 1 || updated.Payments[0].Amount != money.Naira(5_000) {
		t.Fatalf("existing next invoice payments = %+v", updated.Payments)
	}
	if updated.Balance() != money.Naira(30_000) {
		t.Fatalf("next invoice balance = %v", updated.Balance())
	}
}
