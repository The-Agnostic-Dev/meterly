package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/palmgrove-estates/meterly/internal/meters"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

// meterView carries the optional messages shown on the meter page.
type meterView struct {
	BuyError     string
	BuyAmount    string
	ReadingError string
	ReadingDate  string
	ReadingValue string
	Notice       string
}

func newRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) loadMeter(w http.ResponseWriter, r *http.Request) (store.Meter, store.Flat, bool) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return store.Meter{}, store.Flat{}, false
	}
	m, err := s.store.Meter(id)
	if err != nil {
		s.fail(w, r, err)
		return store.Meter{}, store.Flat{}, false
	}
	f, err := s.store.Flat(m.FlatID)
	if err != nil || !canSeeFlat(currentUser(r), f.ID) {
		s.notFound(w, r)
		return store.Meter{}, store.Flat{}, false
	}
	return m, f, true
}

func (s *Server) meterPage(w http.ResponseWriter, r *http.Request, status int, m store.Meter, f store.Flat, v meterView) {
	band, _ := s.store.Band(m.Band)
	u := currentUser(r)
	s.render(w, r, status, "meter.html", "Meter "+m.Serial, map[string]any{
		"Meter":     m,
		"Flat":      f,
		"Band":      band,
		"Low":       m.Balance < meters.LowBalance,
		"CanBuy":    u.Role == store.RoleManager || u.Role == store.RoleResident,
		"RequestID": newRequestID(),
		"Min":       meters.MinPurchase,
		"Max":       meters.MaxPurchase,
		"Recent":    s.meters.Recent(m.ID, 5),
		"Monthly":   s.readings.Monthly(m.ID, 6),
		"Readings":  s.readings.History(m.ID),
		"View":      v,
	})
}

func (s *Server) meterDetail(w http.ResponseWriter, r *http.Request) {
	m, f, ok := s.loadMeter(w, r)
	if !ok {
		return
	}
	var v meterView
	switch r.URL.Query().Get("done") {
	case "reading-added":
		v.Notice = "Reading saved."
	case "reading-corrected":
		v.Notice = "Reading corrected."
	case "reading-removed":
		v.Notice = "Reading removed."
	}
	s.meterPage(w, r, http.StatusOK, m, f, v)
}

func (s *Server) buyUnits(w http.ResponseWriter, r *http.Request) {
	m, f, ok := s.loadMeter(w, r)
	if !ok {
		return
	}
	raw := r.FormValue("amount")
	amount, err := money.Parse(raw)
	if err == nil {
		var p store.Purchase
		p, err = s.meters.Buy(r.Context(), meters.BuyRequest{
			MeterID:   m.ID,
			Amount:    amount,
			RequestID: r.FormValue("request_id"),
			BoughtBy:  currentUser(r).Name,
		})
		if err == nil {
			http.Redirect(w, r, "/purchases/"+strconv.Itoa(p.ID), http.StatusSeeOther)
			return
		}
	}
	if !errors.Is(err, money.ErrInvalid) && !errors.Is(err, meters.ErrAmount) {
		s.log.Warn("buy units", "meter", m.Serial, "err", err)
	}
	s.meterPage(w, r, http.StatusUnprocessableEntity, m, f, meterView{BuyError: err.Error(), BuyAmount: raw})
}

func (s *Server) refundPurchase(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	if _, err := s.meters.Refund(id, currentUser(r).Name); err != nil {
		if !errors.Is(err, store.ErrAlreadyResolved) {
			s.fail(w, r, err)
			return
		}
	}
	http.Redirect(w, r, "/purchases/"+strconv.Itoa(id), http.StatusSeeOther)
}

func (s *Server) creditPurchase(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	if _, _, err := s.meters.CreditNextMonth(id, currentUser(r).Name); err != nil {
		if !errors.Is(err, store.ErrAlreadyResolved) {
			s.fail(w, r, err)
			return
		}
	}
	http.Redirect(w, r, "/purchases/"+strconv.Itoa(id), http.StatusSeeOther)
}

func (s *Server) receipt(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	p, err := s.store.Purchase(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	m, err := s.store.Meter(p.MeterID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	f, err := s.store.Flat(m.FlatID)
	if err != nil || !canSeeFlat(currentUser(r), f.ID) {
		s.notFound(w, r)
		return
	}
	band, _ := s.store.Band(m.Band)
	s.render(w, r, http.StatusOK, "receipt.html", "Receipt "+p.Ref, map[string]any{
		"Purchase": p, "Meter": m, "Flat": f, "Band": band,
	})
}
