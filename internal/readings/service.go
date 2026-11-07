// Package readings records meter register readings and turns them into
// monthly consumption.
package readings

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/store"
)

// ErrFuture is returned for readings dated after today.
var ErrFuture = errors.New("a reading cannot be dated in the future")

// ErrLower is returned for readings below the meter's most recent reading.
var ErrLower = errors.New("the reading cannot be lower than the last reading, check the figure on the meter")

// Service records readings.
type Service struct {
	store    *store.Store
	onChange []func(month string)
}

// NewService returns a readings service.
func NewService(s *store.Store) *Service { return &Service{store: s} }

// OnChange registers a callback that runs with the month ("2006-01") of any
// reading that is added, corrected or removed.
func (s *Service) OnChange(fn func(month string)) { s.onChange = append(s.onChange, fn) }

func (s *Service) changed(date time.Time) {
	month := clock.MonthKey(date)
	for _, fn := range s.onChange {
		fn(month)
	}
}

// Add records a new reading for a meter.
func (s *Service) Add(meterID int, date time.Time, value energy.KWh, by string) (store.Reading, error) {
	if _, err := s.store.Meter(meterID); err != nil {
		return store.Reading{}, err
	}
	date = clock.StartOfDay(date)
	if date.After(clock.Today()) {
		return store.Reading{}, ErrFuture
	}
	history := s.store.Readings(meterID)
	if len(history) > 0 {
		last := history[len(history)-1]
		if date.Before(last.Date) {
			return store.Reading{}, fmt.Errorf("there is already a reading dated %s, new readings must be on or after it", last.Date.Format("02 Jan 2006"))
		}
		if value < last.Value {
			return store.Reading{}, ErrLower
		}
	}
	r := s.store.AddReading(store.Reading{MeterID: meterID, Date: date, Value: value, TakenBy: by})
	s.changed(r.Date)
	return r, nil
}

// Correct changes the value of an existing reading. The new value must still
// sit between the readings before and after it.
func (s *Service) Correct(id int, value energy.KWh, by string) (store.Reading, error) {
	r, err := s.store.Reading(id)
	if err != nil {
		return store.Reading{}, err
	}
	history := s.store.Readings(r.MeterID)
	i := slices.IndexFunc(history, func(h store.Reading) bool { return h.ID == id })
	if i > 0 && value < history[i-1].Value {
		return store.Reading{}, fmt.Errorf("%s kWh is lower than the reading before it (%s kWh on %s)",
			value, history[i-1].Value, history[i-1].Date.Format("02 Jan 2006"))
	}
	if i >= 0 && i < len(history)-1 && value > history[i+1].Value {
		return store.Reading{}, fmt.Errorf("%s kWh is higher than the reading after it (%s kWh on %s)",
			value, history[i+1].Value, history[i+1].Date.Format("02 Jan 2006"))
	}
	updated, err := s.store.UpdateReading(id, value, by)
	if err != nil {
		return store.Reading{}, err
	}
	s.changed(updated.Date)
	return updated, nil
}

// Remove deletes a reading.
func (s *Service) Remove(id int) (store.Reading, error) {
	r, err := s.store.DeleteReading(id)
	if err != nil {
		return store.Reading{}, err
	}
	s.changed(r.Date)
	return r, nil
}

// MonthUsage is the consumption of one meter in one month.
type MonthUsage struct {
	Month       string
	Opening     energy.KWh
	Closing     energy.KWh
	Consumption energy.KWh
	Known       bool
}

// Consumption works out a meter's usage in a month: the last reading in the
// month minus the last reading before the month started.
func Consumption(history []store.Reading, month string) MonthUsage {
	u := MonthUsage{Month: month}
	start, err := clock.ParseMonth(month)
	if err != nil {
		return u
	}
	end := start.AddDate(0, 1, 0)
	var opening, closing *store.Reading
	for i := range history {
		r := &history[i]
		switch {
		case r.Date.Before(start):
			opening = r
		case r.Date.Before(end):
			closing = r
		}
	}
	if opening == nil || closing == nil {
		return u
	}
	u.Opening, u.Closing = opening.Value, closing.Value
	u.Consumption = closing.Value - opening.Value
	u.Known = true
	return u
}

// Monthly returns the last n months of usage for a meter, newest first.
func (s *Service) Monthly(meterID, n int) []MonthUsage {
	history := s.store.Readings(meterID)
	out := make([]MonthUsage, 0, n)
	for _, m := range clock.RecentMonths(n) {
		out = append(out, Consumption(history, m))
	}
	return out
}

// History lists a meter's readings newest first.
func (s *Service) History(meterID int) []store.Reading {
	rs := s.store.Readings(meterID)
	slices.Reverse(rs)
	return rs
}
