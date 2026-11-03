package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/palmgrove-estates/meterly/internal/billing"
	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/store"
)

type flatRow struct {
	Flat  store.Flat
	Meter store.Meter
}

func (s *Server) flatList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	needle := strings.ToLower(q)
	var rows []flatRow
	for _, f := range s.store.Flats() {
		if needle != "" && !strings.Contains(strings.ToLower(f.Resident+" "+f.Label()+" "+f.Phone), needle) {
			continue
		}
		m, _ := s.store.Meter(f.MeterID)
		rows = append(rows, flatRow{Flat: f, Meter: m})
	}
	s.render(w, r, http.StatusOK, "flats.html", "Flats & residents", map[string]any{"Rows": rows, "Query": q})
}

func (s *Server) flatPage(w http.ResponseWriter, r *http.Request, status int, f store.Flat, form *residentForm, saved bool) {
	m, err := s.store.Meter(f.MeterID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	band, _ := s.store.Band(m.Band)
	var invoices []billing.Line
	for _, l := range s.billing.Lines("", "") {
		if l.FlatID == f.ID {
			invoices = append(invoices, l)
		}
	}
	if form == nil {
		form = &residentForm{Name: f.Resident, Phone: f.Phone, Email: f.Email}
	}
	s.render(w, r, status, "flat.html", "Flat "+f.Label(), map[string]any{
		"Flat":     f,
		"Meter":    m,
		"Band":     band,
		"Charge":   billing.Charges[f.Type],
		"Invoices": invoices,
		"Form":     form,
		"Saved":    saved,
		"Today":    clock.Today(),
	})
}

func (s *Server) flatDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok || !canSeeFlat(currentUser(r), id) {
		s.notFound(w, r)
		return
	}
	f, err := s.store.Flat(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.flatPage(w, r, http.StatusOK, f, nil, r.URL.Query().Get("saved") == "1")
}

func (s *Server) updateResident(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	f, err := s.store.Flat(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	form := &residentForm{Name: r.FormValue("name"), Phone: r.FormValue("phone"), Email: r.FormValue("email")}
	if !form.validate() {
		s.flatPage(w, r, http.StatusUnprocessableEntity, f, form, false)
		return
	}
	if err := s.store.UpdateResident(id, form.Name, form.Phone, form.Email); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/flats/"+strconv.Itoa(id)+"?saved=1", http.StatusSeeOther)
}
