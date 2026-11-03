package web

import (
	"net/http"
	"strconv"

	"github.com/palmgrove-estates/meterly/internal/billing"
	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/meters"
	"github.com/palmgrove-estates/meterly/internal/store"
)

type lowMeter struct {
	Flat  store.Flat
	Meter store.Meter
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.Role == store.RoleResident {
		f, err := s.store.Flat(u.FlatID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		http.Redirect(w, r, "/meters/"+strconv.Itoa(f.MeterID), http.StatusSeeOther)
		return
	}

	flats := s.store.Flats()
	var low []lowMeter
	for _, f := range flats {
		m, err := s.store.Meter(f.MeterID)
		if err == nil && m.Balance < meters.LowBalance {
			low = append(low, lowMeter{Flat: f, Meter: m})
		}
	}
	monthStart := clock.StartOfMonth(clock.Now())
	sales, units, count := s.meters.SalesBetween(monthStart, monthStart.AddDate(0, 1, 0))
	_, _, outstanding := billing.Totals(s.billing.Lines("", "outstanding"))

	latest := s.meters.History(0)
	if len(latest) > 6 {
		latest = latest[:6]
	}
	s.render(w, r, http.StatusOK, "dashboard.html", "Dashboard", map[string]any{
		"Flats":       len(flats),
		"Low":         low,
		"LowLimit":    meters.LowBalance,
		"Month":       clock.MonthKey(monthStart),
		"Sales":       sales,
		"SalesUnits":  units,
		"SalesCount":  count,
		"Outstanding": outstanding,
		"Latest":      s.purchaseRows(latest),
	})
}

// purchaseRow is a purchase with the flat it was for.
type purchaseRow struct {
	store.Purchase
	Flat store.Flat
}

func (s *Server) purchaseRows(ps []store.Purchase) []purchaseRow {
	flatByMeter := map[int]store.Flat{}
	for _, f := range s.store.Flats() {
		flatByMeter[f.MeterID] = f
	}
	out := make([]purchaseRow, len(ps))
	for i, p := range ps {
		out[i] = purchaseRow{Purchase: p, Flat: flatByMeter[p.MeterID]}
	}
	return out
}
