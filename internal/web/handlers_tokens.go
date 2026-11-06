package web

import (
	"net/http"
	"strconv"

	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

const tokensPerPage = 10

func (s *Server) tokenHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	meterID, _ := strconv.Atoi(q.Get("meter"))
	page, _ := strconv.Atoi(q.Get("page"))

	all := s.meters.History(meterID)
	var total money.Kobo
	for _, p := range all {
		total += p.Amount
	}
	pg := NewPager(len(all), page, tokensPerPage)
	var rows []store.Purchase
	if first, last := pg.Window(); first <= last {
		rows = all[first : last+1]
	}

	var selected store.Flat
	flats := s.store.Flats()
	for _, f := range flats {
		if f.MeterID == meterID {
			selected = f
		}
	}
	s.render(w, r, http.StatusOK, "tokens.html", "Token history", map[string]any{
		"Rows":     s.purchaseRows(rows),
		"Pager":    pg,
		"Total":    total,
		"Flats":    flats,
		"MeterID":  meterID,
		"Selected": selected,
	})
}
