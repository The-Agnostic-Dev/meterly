// Package billing raises monthly service charge invoices and records payments.
package billing

import (
	"errors"
	"fmt"
	"time"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

// Monthly service charge by flat type, approved at the 2026 residents' AGM.
var Charges = map[string]money.Kobo{
	"1-bed":           money.Naira(25_000),
	"2-bed":           money.Naira(35_000),
	"3-bed":           money.Naira(45_000),
	"4-bed penthouse": money.Naira(60_000),
}

// DueDay is the day of the month service charge falls due.
const DueDay = 10

// Invoice statuses.
const (
	StatusPaid     = "paid"
	StatusPartPaid = "part-paid"
	StatusUnpaid   = "unpaid"
	StatusOverdue  = "overdue"
)

// Status describes where an invoice stands on a given day.
func Status(inv store.Invoice, today time.Time) string {
	switch {
	case inv.Balance() <= 0:
		return StatusPaid
	case today.After(inv.DueOn):
		return StatusOverdue
	case inv.Paid() > 0:
		return StatusPartPaid
	default:
		return StatusUnpaid
	}
}

// Line is an invoice with its flat, ready for display.
type Line struct {
	store.Invoice
	Flat   store.Flat
	Status string
}

// Service handles invoices.
type Service struct{ store *store.Store }

// NewService returns a billing service.
func NewService(s *store.Store) *Service { return &Service{store: s} }

// Lines lists invoices for a month. status filters by Status ("" means all,
// "outstanding" means anything not fully paid).
func (s *Service) Lines(month, status string) []Line {
	today := clock.Today()
	var out []Line
	for _, inv := range s.store.Invoices() {
		if month != "" && inv.Month != month {
			continue
		}
		f, err := s.store.Flat(inv.FlatID)
		if err != nil {
			continue
		}
		st := Status(inv, today)
		switch status {
		case "":
		case "outstanding":
			if st == StatusPaid {
				continue
			}
		default:
			if st != status {
				continue
			}
		}
		out = append(out, Line{Invoice: inv, Flat: f, Status: st})
	}
	return out
}

// Line returns one invoice with its flat.
func (s *Service) Line(id int) (Line, error) {
	inv, err := s.store.Invoice(id)
	if err != nil {
		return Line{}, err
	}
	f, err := s.store.Flat(inv.FlatID)
	if err != nil {
		return Line{}, err
	}
	return Line{Invoice: inv, Flat: f, Status: Status(inv, clock.Today())}, nil
}

// Totals adds up a set of lines.
func Totals(lines []Line) (billed, received, outstanding money.Kobo) {
	for _, l := range lines {
		billed += l.Amount
		received += l.Paid()
		outstanding += l.Balance()
	}
	return billed, received, outstanding
}

// ErrPaymentAmount is returned for a zero or negative payment.
var ErrPaymentAmount = errors.New("enter the amount received")

// RecordPayment records money received against an invoice. Payments larger
// than the balance are refused; the accountant records overpayments as credit
// on the next invoice instead.
func (s *Service) RecordPayment(invoiceID int, amount money.Kobo, receivedOn time.Time, by string) (store.Invoice, error) {
	if amount <= 0 {
		inv, err := s.store.Invoice(invoiceID)
		if err != nil {
			return store.Invoice{}, err
		}
		return inv, ErrPaymentAmount
	}
	if receivedOn.After(clock.Today()) {
		inv, _ := s.store.Invoice(invoiceID)
		return inv, errors.New("the payment date cannot be in the future")
	}
	return s.store.AddPayment(invoiceID, store.Payment{Amount: amount, ReceivedOn: receivedOn, RecordedBy: by}, func(inv store.Invoice) error {
		if amount > inv.Balance() {
			return fmt.Errorf("%s is more than the outstanding balance of %s", amount, inv.Balance())
		}
		return nil
	})
}
