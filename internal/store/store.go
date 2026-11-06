// Package store is Meterly's in-memory data store. It is seeded with demo
// data on start and guarded by a single RWMutex.
package store

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/money"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// ErrAlreadyResolved is returned when a purchase was already refunded or credited.
var ErrAlreadyResolved = errors.New("purchase already resolved")

// Store holds every record. All methods return copies.
type Store struct {
	mu        sync.RWMutex
	users     []User
	bands     map[string]Band
	flats     map[int]*Flat
	meters    map[int]*Meter
	purchases []*Purchase
	readings  map[int]*Reading
	invoices  map[int]*Invoice
	nextID    int
}

// New returns an empty store.
func New() *Store {
	return &Store{
		bands:    map[string]Band{},
		flats:    map[int]*Flat{},
		meters:   map[int]*Meter{},
		readings: map[int]*Reading{},
		invoices: map[int]*Invoice{},
		nextID:   1000,
	}
}

func (s *Store) id() int {
	s.nextID++
	return s.nextID
}

// UserByEmail finds a user by sign-in email.
func (s *Store) UserByEmail(email string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email == email {
			return u, true
		}
	}
	return User{}, false
}

// User finds a user by id.
func (s *Store) User(id int) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

// Users lists every user.
func (s *Store) Users() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.users)
}

// Band returns a tariff band by code.
func (s *Store) Band(code string) (Band, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.bands[code]
	return b, ok
}

// Flats lists every flat ordered by block and number.
func (s *Store) Flats() []Flat {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Flat, 0, len(s.flats))
	for _, f := range s.flats {
		out = append(out, *f)
	}
	slices.SortFunc(out, func(a, b Flat) int {
		if a.Block != b.Block {
			if a.Block < b.Block {
				return -1
			}
			return 1
		}
		return a.Number - b.Number
	})
	return out
}

// Flat returns one flat.
func (s *Store) Flat(id int) (Flat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.flats[id]
	if !ok {
		return Flat{}, ErrNotFound
	}
	return *f, nil
}

// UpdateResident saves the household details of a flat.
func (s *Store) UpdateResident(id int, name, phone, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flats[id]
	if !ok {
		return ErrNotFound
	}
	f.Resident, f.Phone, f.Email = name, phone, email
	return nil
}

// Meter returns one meter.
func (s *Store) Meter(id int) (Meter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.meters[id]
	if !ok {
		return Meter{}, ErrNotFound
	}
	return *m, nil
}

// Meters lists every meter.
func (s *Store) Meters() []Meter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Meter, 0, len(s.meters))
	for _, m := range s.meters {
		out = append(out, *m)
	}
	slices.SortFunc(out, func(a, b Meter) int { return a.ID - b.ID })
	return out
}

// Purchases lists every purchase in the order they were saved.
func (s *Store) Purchases() []Purchase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Purchase, 0, len(s.purchases))
	for _, p := range s.purchases {
		out = append(out, *p)
	}
	return out
}

// Purchase returns one purchase.
func (s *Store) Purchase(id int) (Purchase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.purchases {
		if p.ID == id {
			return *p, nil
		}
	}
	return Purchase{}, ErrNotFound
}

// PurchaseByRequest finds a purchase by the request id the buy form sent.
func (s *Store) PurchaseByRequest(requestID string) (Purchase, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.purchases {
		if p.RequestID == requestID {
			return *p, true
		}
	}
	return Purchase{}, false
}

// AddPurchase saves a purchase and credits its units to the meter. A non-empty
// request id that is already saved returns the original purchase without
// crediting the meter again.
func (s *Store) AddPurchase(p Purchase) (Purchase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.RequestID != "" {
		for _, existing := range s.purchases {
			if existing.RequestID == p.RequestID {
				return *existing, nil
			}
		}
	}
	m, ok := s.meters[p.MeterID]
	if !ok {
		return Purchase{}, ErrNotFound
	}
	p.ID = s.id()
	p.Ref = fmt.Sprintf("MTR-%06d", p.ID)
	m.Balance += p.Units
	s.purchases = append(s.purchases, &p)
	return p, nil
}

func (s *Store) purchaseLocked(id int) (*Purchase, bool) {
	for _, p := range s.purchases {
		if p.ID == id {
			return p, true
		}
	}
	return nil, false
}

// RefundPurchase reverses the units of a purchase and records the refund.
func (s *Store) RefundPurchase(id int, by string, at time.Time) (Purchase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.purchaseLocked(id)
	if !ok {
		return Purchase{}, ErrNotFound
	}
	if p.Refunded || p.Credited {
		return *p, ErrAlreadyResolved
	}
	m, ok := s.meters[p.MeterID]
	if !ok {
		return *p, ErrNotFound
	}
	m.Balance -= p.Units
	p.Refunded = true
	p.ResolvedBy = by
	p.ResolvedAt = at
	return *p, nil
}

// CreditPurchase reverses the units of a purchase and applies its amount to
// the flat's invoice for month, creating the invoice at charge when missing.
func (s *Store) CreditPurchase(id int, month string, charge money.Kobo, issuedOn, dueOn time.Time, by string, at time.Time) (Purchase, Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.purchaseLocked(id)
	if !ok {
		return Purchase{}, Invoice{}, ErrNotFound
	}
	if p.Refunded || p.Credited {
		return *p, Invoice{}, ErrAlreadyResolved
	}
	m, ok := s.meters[p.MeterID]
	if !ok {
		return *p, Invoice{}, ErrNotFound
	}
	f, ok := s.flats[m.FlatID]
	if !ok {
		return *p, Invoice{}, ErrNotFound
	}
	var inv *Invoice
	for _, candidate := range s.invoices {
		if candidate.FlatID == f.ID && candidate.Month == month {
			inv = candidate
			break
		}
	}
	if inv == nil {
		inv = &Invoice{
			ID:       s.id(),
			FlatID:   f.ID,
			Month:    month,
			Amount:   charge,
			IssuedOn: issuedOn,
			DueOn:    dueOn,
		}
		s.invoices[inv.ID] = inv
	}
	m.Balance -= p.Units
	p.Credited = true
	p.ResolvedBy = by
	p.ResolvedAt = at
	p.CreditInvoiceID = inv.ID
	inv.Payments = append(inv.Payments, Payment{
		ID:         s.id(),
		Amount:     p.Amount,
		ReceivedOn: at,
		RecordedBy: by,
	})
	c := *inv
	c.Payments = slices.Clone(inv.Payments)
	return *p, c, nil
}

// Readings lists a meter's readings, oldest first.
func (s *Store) Readings(meterID int) []Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Reading
	for _, r := range s.readings {
		if r.MeterID == meterID {
			out = append(out, *r)
		}
	}
	slices.SortFunc(out, func(a, b Reading) int { return a.Date.Compare(b.Date) })
	return out
}

// Reading returns one reading.
func (s *Store) Reading(id int) (Reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.readings[id]
	if !ok {
		return Reading{}, ErrNotFound
	}
	return *r, nil
}

// AddReading saves a new reading.
func (s *Store) AddReading(r Reading) Reading {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = s.id()
	r.UpdatedAt = time.Now()
	s.readings[r.ID] = &r
	return r
}

// UpdateReading changes the value of a reading.
func (s *Store) UpdateReading(id int, value energy.KWh, by string) (Reading, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.readings[id]
	if !ok {
		return Reading{}, ErrNotFound
	}
	r.Value = value
	r.TakenBy = by
	r.UpdatedAt = time.Now()
	return *r, nil
}

// DeleteReading removes a reading.
func (s *Store) DeleteReading(id int) (Reading, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.readings[id]
	if !ok {
		return Reading{}, ErrNotFound
	}
	delete(s.readings, id)
	return *r, nil
}

// Invoices lists every invoice.
func (s *Store) Invoices() []Invoice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Invoice, 0, len(s.invoices))
	for _, inv := range s.invoices {
		c := *inv
		c.Payments = slices.Clone(inv.Payments)
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b Invoice) int { return a.ID - b.ID })
	return out
}

// Invoice returns one invoice.
func (s *Store) Invoice(id int) (Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, ok := s.invoices[id]
	if !ok {
		return Invoice{}, ErrNotFound
	}
	c := *inv
	c.Payments = slices.Clone(inv.Payments)
	return c, nil
}

// AddPayment records a payment against an invoice. The check function runs
// under the write lock with the current invoice so callers can validate atomically.
func (s *Store) AddPayment(invoiceID int, p Payment, check func(Invoice) error) (Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invoices[invoiceID]
	if !ok {
		return Invoice{}, ErrNotFound
	}
	if check != nil {
		if err := check(*inv); err != nil {
			return *inv, err
		}
	}
	p.ID = s.id()
	inv.Payments = append(inv.Payments, p)
	c := *inv
	c.Payments = slices.Clone(inv.Payments)
	return c, nil
}

// Load replaces the store contents. Used by the seed.
func (s *Store) Load(users []User, bands []Band, flats []Flat, meters []Meter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = users
	for _, b := range bands {
		s.bands[b.Code] = b
	}
	for i := range flats {
		f := flats[i]
		s.flats[f.ID] = &f
	}
	for i := range meters {
		m := meters[i]
		s.meters[m.ID] = &m
	}
}

// AddInvoice saves a new invoice.
func (s *Store) AddInvoice(inv Invoice) Invoice {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv.ID = s.id()
	for i := range inv.Payments {
		inv.Payments[i].ID = s.id()
	}
	s.invoices[inv.ID] = &inv
	return inv
}

// SetBalance overwrites the units left on a meter, as reported by the meter itself.
func (s *Store) SetBalance(meterID int, balance energy.KWh) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.meters[meterID]
	if !ok {
		return ErrNotFound
	}
	m.Balance = balance
	return nil
}
