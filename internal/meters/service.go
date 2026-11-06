// Package meters sells units: it turns a naira amount into kWh at the meter's
// tariff band, asks the vendor for a token and credits the meter.
package meters

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/palmgrove-estates/meterly/internal/billing"
	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

// Purchase limits for a single top-up.
var (
	MinPurchase = money.Naira(1_000)
	MaxPurchase = money.Naira(500_000)
)

// LowBalance is the level below which a meter is flagged on the dashboard.
const LowBalance = energy.KWh(20_00)

// Quote is the breakdown of a top-up amount.
type Quote struct {
	Amount money.Kobo
	Net    money.Kobo
	VAT    money.Kobo
	Rate   money.Kobo
	Units  energy.KWh
}

// NewQuote splits the VAT out of amount and converts the rest to units at rate per kWh.
func NewQuote(amount, rate money.Kobo) Quote {
	net, vat := money.SplitVAT(amount)
	q := Quote{Amount: amount, Net: net, VAT: vat, Rate: rate}
	if rate > 0 {
		q.Units = energy.KWh(int64(net) * 100 / int64(rate))
	}
	return q
}

// BuyRequest is a top-up asked for from the buy form.
type BuyRequest struct {
	MeterID   int
	Amount    money.Kobo
	RequestID string
	BoughtBy  string
}

// Service sells units.
type Service struct {
	store  *store.Store
	vendor Vendor
}

// NewService returns a meters service.
func NewService(s *store.Store, v Vendor) *Service {
	return &Service{store: s, vendor: v}
}

// ErrAmount is returned when the amount is outside the purchase limits.
var ErrAmount = fmt.Errorf("top-ups must be between %s and %s", MinPurchase, MaxPurchase)

// Buy sells units to a meter and returns the saved purchase. A request id
// that was already used returns the original purchase, so a form submitted
// twice does not charge the resident twice.
func (s *Service) Buy(ctx context.Context, req BuyRequest) (store.Purchase, error) {
	if req.Amount < MinPurchase || req.Amount > MaxPurchase {
		return store.Purchase{}, ErrAmount
	}
	if req.RequestID != "" {
		if p, ok := s.store.PurchaseByRequest(req.RequestID); ok {
			return p, nil
		}
	}
	meter, err := s.store.Meter(req.MeterID)
	if err != nil {
		return store.Purchase{}, err
	}
	band, ok := s.store.Band(meter.Band)
	if !ok {
		return store.Purchase{}, fmt.Errorf("meter %s has unknown tariff band %q", meter.Serial, meter.Band)
	}
	q := NewQuote(req.Amount, band.Rate)
	token, err := s.vendor.Issue(ctx, meter.Serial, q.Units)
	if err != nil {
		return store.Purchase{}, errors.Join(errors.New("the vending gateway did not issue a token, you have not been charged"), err)
	}
	return s.store.AddPurchase(store.Purchase{
		MeterID:   meter.ID,
		Amount:    q.Amount,
		Net:       q.Net,
		VAT:       q.VAT,
		Rate:      q.Rate,
		Units:     q.Units,
		Token:     token,
		RequestID: req.RequestID,
		BoughtBy:  req.BoughtBy,
		CreatedAt: clock.Now(),
	})
}

// Refund reverses a purchase and records the manager who resolved it.
func (s *Service) Refund(purchaseID int, by string) (store.Purchase, error) {
	return s.store.RefundPurchase(purchaseID, by, clock.Now())
}

// CreditNextMonth reverses a purchase and applies its amount to the flat's
// service-charge invoice for the month after this one.
func (s *Service) CreditNextMonth(purchaseID int, by string) (store.Purchase, store.Invoice, error) {
	p, err := s.store.Purchase(purchaseID)
	if err != nil {
		return store.Purchase{}, store.Invoice{}, err
	}
	m, err := s.store.Meter(p.MeterID)
	if err != nil {
		return store.Purchase{}, store.Invoice{}, err
	}
	f, err := s.store.Flat(m.FlatID)
	if err != nil {
		return store.Purchase{}, store.Invoice{}, err
	}
	next := clock.StartOfMonth(clock.Today()).AddDate(0, 1, 0)
	charge := billing.Charges[f.Type]
	if charge == 0 {
		charge = p.Amount
	}
	issuedOn := clock.Today()
	dueOn := clock.StartOfDay(next.AddDate(0, 0, billing.DueDay-1))
	return s.store.CreditPurchase(purchaseID, clock.MonthKey(next), charge, issuedOn, dueOn, by)
}

// History lists purchases newest first. meterID 0 means every meter.
func (s *Service) History(meterID int) []store.Purchase {
	var out []store.Purchase
	for _, p := range s.store.Purchases() {
		if meterID == 0 || p.MeterID == meterID {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, newestFirst)
	return out
}

// Recent returns the latest n purchases for a meter.
func (s *Service) Recent(meterID, n int) []store.Purchase {
	var out []store.Purchase
	for _, p := range s.store.Purchases() {
		if p.MeterID == meterID {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b store.Purchase) int { return a.CreatedAt.Compare(b.CreatedAt) })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func newestFirst(a, b store.Purchase) int { return b.CreatedAt.Compare(a.CreatedAt) }

// SalesBetween sums purchases made in [from, to).
func (s *Service) SalesBetween(from, to time.Time) (money.Kobo, energy.KWh, int) {
	var amount money.Kobo
	var units energy.KWh
	n := 0
	for _, p := range s.store.Purchases() {
		if !p.CreatedAt.Before(from) && p.CreatedAt.Before(to) {
			amount += p.Amount
			units += p.Units
			n++
		}
	}
	return amount, units, n
}
