package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/readings"
)

func (s *Server) addReading(w http.ResponseWriter, r *http.Request) {
	m, f, ok := s.loadMeter(w, r)
	if !ok {
		return
	}
	v := meterView{ReadingDate: r.FormValue("date"), ReadingValue: r.FormValue("value")}
	date, err := clock.ParseDate(v.ReadingDate)
	if err != nil {
		v.ReadingError = "Pick the date the reading was taken."
		s.meterPage(w, r, http.StatusUnprocessableEntity, m, f, v)
		return
	}
	value, err := energy.Parse(v.ReadingValue)
	if err == nil {
		_, err = s.readings.Add(m.ID, date, value, currentUser(r).Name)
	}
	if err != nil {
		v.ReadingError = err.Error()
		status := http.StatusUnprocessableEntity
		if errors.Is(err, readings.ErrLower) {
			status = http.StatusBadRequest
		}
		s.meterPage(w, r, status, m, f, v)
		return
	}
	http.Redirect(w, r, "/meters/"+strconv.Itoa(m.ID)+"?done=reading-added", http.StatusSeeOther)
}

func (s *Server) correctReading(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	existing, err := s.store.Reading(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	value, err := energy.Parse(r.FormValue("value"))
	if err == nil {
		_, err = s.readings.Correct(id, value, currentUser(r).Name)
	}
	if err != nil {
		m, _ := s.store.Meter(existing.MeterID)
		f, _ := s.store.Flat(m.FlatID)
		s.meterPage(w, r, http.StatusUnprocessableEntity, m, f, meterView{ReadingError: err.Error()})
		return
	}
	http.Redirect(w, r, "/meters/"+strconv.Itoa(existing.MeterID)+"?done=reading-corrected", http.StatusSeeOther)
}

func (s *Server) removeReading(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	removed, err := s.readings.Remove(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/meters/"+strconv.Itoa(removed.MeterID)+"?done=reading-removed", http.StatusSeeOther)
}
