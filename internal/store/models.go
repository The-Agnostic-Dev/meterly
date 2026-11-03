package store

import (
	"time"

	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/money"
)

// Role decides what a signed-in user can see.
type Role string

const (
	RoleManager    Role = "manager"
	RoleAccountant Role = "accountant"
	RoleResident   Role = "resident"
)

// User is someone who can sign in to Meterly.
type User struct {
	ID       int
	Name     string
	Email    string
	Password string
	Role     Role
	FlatID   int // residents only
}

// Band is a DisCo tariff band. Rate is the price of one kWh before VAT.
type Band struct {
	Code   string
	Rate   money.Kobo
	Supply string
}

// Flat is one apartment in the estate and the household living in it.
type Flat struct {
	ID       int
	Block    string
	Number   int
	Type     string
	Resident string
	Phone    string
	Email    string
	MeterID  int
}

// Label is the short name used around the estate, e.g. "B4".
func (f Flat) Label() string { return f.Block + itoa(f.Number) }

// Meter is the prepaid meter installed in a flat.
type Meter struct {
	ID      int
	Serial  string
	FlatID  int
	Band    string
	Balance energy.KWh
}

// Purchase is a top-up: money paid, VAT, units credited and the STS token issued.
type Purchase struct {
	ID        int
	Ref       string
	MeterID   int
	Amount    money.Kobo
	Net       money.Kobo
	VAT       money.Kobo
	Rate      money.Kobo
	Units     energy.KWh
	Token     string
	RequestID string
	BoughtBy  string
	CreatedAt time.Time
}

// Reading is a cumulative register reading taken on a given day.
type Reading struct {
	ID        int
	MeterID   int
	Date      time.Time
	Value     energy.KWh
	TakenBy   string
	UpdatedAt time.Time
}

// Invoice is a monthly service charge bill for a flat.
type Invoice struct {
	ID       int
	FlatID   int
	Month    string
	Amount   money.Kobo
	IssuedOn time.Time
	DueOn    time.Time
	Payments []Payment
}

// Paid is the sum of payments received against the invoice.
func (i Invoice) Paid() money.Kobo {
	var total money.Kobo
	for _, p := range i.Payments {
		total += p.Amount
	}
	return total
}

// Balance is what is still owed.
func (i Invoice) Balance() money.Kobo { return i.Amount - i.Paid() }

// Payment is money received against a service charge invoice.
type Payment struct {
	ID         int
	Amount     money.Kobo
	ReceivedOn time.Time
	RecordedBy string
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
