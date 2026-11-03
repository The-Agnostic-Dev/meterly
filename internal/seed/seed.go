// Package seed fills the store with Palmgrove Court demo data. Dates are
// relative to today so the demo always looks current.
package seed

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/palmgrove-estates/meterly/internal/billing"
	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/meters"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

// DemoPassword is shared by every demo account.
const DemoPassword = "meterly2026"

type household struct {
	block    string
	number   int
	kind     string
	band     string
	resident string
	phone    string
	email    string
	daily    float64 // typical kWh per day
}

var households = []household{
	{"A", 1, "3-bed", "A", "Adebayo Ogunleye", "08031234501", "adebayo.ogunleye@gmail.com", 14.5},
	{"A", 2, "3-bed", "A", "Chioma Okafor", "08052214477", "chioma.okafor@gmail.com", 17.2},
	{"A", 3, "3-bed", "A", "Emeka Nwosu", "08094410082", "emeka.nwosu@yahoo.com", 11.8},
	{"A", 4, "3-bed", "A", "Funmilayo Adeyemi", "07036672210", "funmi.adeyemi@gmail.com", 15.4},
	{"A", 5, "3-bed", "A", "Yusuf Abdullahi", "08127783019", "yabdullahi@outlook.com", 12.1},
	{"A", 6, "3-bed", "A", "Kemi Balogun", "09021145563", "kemi.balogun@gmail.com", 18.6},
	{"B", 1, "2-bed", "B", "Ifeoma Obi", "08065512290", "ifeoma.obi@gmail.com", 7.3},
	{"B", 2, "2-bed", "B", "Segun Akinola", "08023390146", "segun.akinola@gmail.com", 9.7},
	{"B", 3, "2-bed", "B", "Halima Bello", "07065128834", "halima.bello@yahoo.com", 6.1},
	{"B", 4, "2-bed", "B", "Ibrahim Musa", "08139904127", "ibrahim.musa@gmail.com", 10.2},
	{"B", 5, "2-bed", "B", "Tolu Fashola", "08034471205", "tolu.fashola@gmail.com", 8.6},
	{"B", 6, "2-bed", "B", "Uche Eze", "09087723361", "uche.eze@gmail.com", 8.4},
	{"C", 1, "1-bed", "B", "Bisi Lawal", "08051187742", "bisi.lawal@gmail.com", 2.8},
	{"C", 2, "1-bed", "B", "Daniel Okon", "07088342219", "daniel.okon@gmail.com", 3.6},
	{"C", 3, "1-bed", "B", "Amaka Udeh", "08166620934", "amaka.udeh@gmail.com", 4.1},
	{"C", 4, "1-bed", "B", "Gbenga Ajayi", "08029915580", "gbenga.ajayi@yahoo.com", 3.1},
	{"C", 5, "4-bed penthouse", "A", "Olumide Coker", "08033308812", "olumide.coker@gmail.com", 38.2},
	{"C", 6, "4-bed penthouse", "A", "Nkechi Okonkwo", "08174456021", "nkechi.okonkwo@gmail.com", 40.5},
}

// Bands are the DisCo tariff bands that apply to the estate's feeders.
var Bands = []store.Band{
	{Code: "A", Rate: 20950, Supply: "at least 20 hours a day"},
	{Code: "B", Rate: 6330, Supply: "at least 16 hours a day"},
	{Code: "C", Rate: 5030, Supply: "at least 12 hours a day"},
}

// Load seeds s with the demo estate.
func Load(s *store.Store) {
	rng := rand.New(rand.NewPCG(2026, 11))
	today := clock.Today()

	users := []store.User{
		{ID: 1, Name: "Tunde Bakare", Email: "fm@palmgrovecourt.ng", Password: DemoPassword, Role: store.RoleManager},
		{ID: 2, Name: "Ngozi Eze", Email: "accounts@palmgrovecourt.ng", Password: DemoPassword, Role: store.RoleAccountant},
	}
	var flats []store.Flat
	var meterList []store.Meter
	for i, h := range households {
		flatID, meterID := 101+i, 501+i
		flats = append(flats, store.Flat{ID: flatID, Block: h.block, Number: h.number, Type: h.kind,
			Resident: h.resident, Phone: h.phone, Email: h.email, MeterID: meterID})
		balance := 5 + rng.Float64()*170
		if i == 4 || i == 14 {
			balance = 6 + rng.Float64()*8
		}
		meterList = append(meterList, store.Meter{ID: meterID, FlatID: flatID, Band: h.band,
			Serial:  fmt.Sprintf("0471-%04d-%04d", 2200+i*37, rng.IntN(10000)),
			Balance: energy.FromFloat(balance)})
	}
	users = append(users,
		store.User{ID: 3, Name: "Chioma Okafor", Email: "chioma.okafor@gmail.com", Password: DemoPassword, Role: store.RoleResident, FlatID: 102},
		store.User{ID: 4, Name: "Ibrahim Musa", Email: "ibrahim.musa@gmail.com", Password: DemoPassword, Role: store.RoleResident, FlatID: 110},
	)
	s.Load(users, Bands, flats, meterList)

	seedPurchases(s, rng, meterList, today)
	for _, m := range meterList {
		s.SetBalance(m.ID, m.Balance)
	}
	seedReadings(s, rng, meterList, today)
	seedInvoices(s, flats, today)
}

func seedPurchases(s *store.Store, rng *rand.Rand, meterList []store.Meter, today time.Time) {
	amounts := map[string][]int64{
		"A": {20_000, 25_000, 30_000, 40_000, 50_000, 75_000, 100_000},
		"B": {5_000, 10_000, 10_000, 15_000, 20_000, 30_000},
	}
	var all []store.Purchase
	for i, m := range meterList {
		n := 3 + rng.IntN(4)
		if i == 0 {
			n = 8
		}
		band, _ := s.Band(m.Band)
		used := map[int]bool{}
		for len(used) < n {
			used[1+rng.IntN(88)] = true
		}
		days := make([]int, 0, n)
		for d := range used {
			days = append(days, d)
		}
		slices.Sort(days)
		for _, daysAgo := range days {
			at := today.AddDate(0, 0, -daysAgo).Add(time.Duration(7+rng.IntN(15))*time.Hour + time.Duration(rng.IntN(60))*time.Minute)
			choices := amounts[m.Band]
			q := meters.NewQuote(money.Naira(choices[rng.IntN(len(choices))]), band.Rate)
			all = append(all, store.Purchase{MeterID: m.ID, Amount: q.Amount, Net: q.Net, VAT: q.VAT, Rate: q.Rate,
				Units: q.Units, Token: meters.FormatToken(rng), BoughtBy: "Resident (bank transfer)", CreatedAt: at})
		}
	}
	slices.SortFunc(all, func(a, b store.Purchase) int { return a.CreatedAt.Compare(b.CreatedAt) })
	for _, p := range all {
		_, _ = s.AddPurchase(p)
	}
}

func seedReadings(s *store.Store, rng *rand.Rand, meterList []store.Meter, today time.Time) {
	thisMonth := clock.StartOfMonth(today)
	for i, m := range meterList {
		daily := households[i].daily
		value := 8000 + float64(i)*731.4 + rng.Float64()*400
		var last time.Time
		for k := 6; k >= 0; k-- {
			date := thisMonth.AddDate(0, -k, 27) // the 28th of each month
			if k == 0 {
				date = today
			}
			if !last.IsZero() {
				days := date.Sub(last).Hours() / 24
				value += daily * days * (0.92 + rng.Float64()*0.16)
			}
			s.AddReading(store.Reading{MeterID: m.ID, Date: date, Value: energy.FromFloat(value), TakenBy: "Tunde Bakare"})
			last = date
		}
	}
}

func seedInvoices(s *store.Store, flats []store.Flat, today time.Time) {
	thisMonth := clock.StartOfMonth(today)
	for k := 2; k >= 0; k-- {
		issued := thisMonth.AddDate(0, -k, 0)
		for i, f := range flats {
			amount := billing.Charges[f.Type]
			var paid money.Kobo
			switch k {
			case 2:
				paid = amount
				if f.Label() == "B3" {
					paid = amount / 2
				}
				if f.Label() == "C4" {
					paid = 0
				}
			case 1:
				switch i % 5 {
				case 0:
					paid = 0
				case 1:
					paid = amount / 2
				default:
					paid = amount
				}
			case 0:
				if i%3 == 0 {
					paid = amount
				}
			}
			inv := store.Invoice{FlatID: f.ID, Month: clock.MonthKey(issued), Amount: amount,
				IssuedOn: issued, DueOn: issued.AddDate(0, 0, billing.DueDay-1)}
			if paid > 0 {
				on := issued.AddDate(0, 0, 1+i%7)
				if on.After(today) {
					on = today
				}
				inv.Payments = []store.Payment{{Amount: paid, ReceivedOn: on, RecordedBy: "Ngozi Eze"}}
			}
			s.AddInvoice(inv)
		}
	}
}
