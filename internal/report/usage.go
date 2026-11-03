// Package report builds the estate-wide usage report.
package report

import (
	"sync"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/readings"
	"github.com/palmgrove-estates/meterly/internal/store"
)

// Row is one flat's line on the report.
type Row struct {
	Flat   store.Flat
	Band   string
	Usage  readings.MonthUsage
	Tokens int
	Bought energy.KWh
	Spend  money.Kobo
}

// Usage is the report for one month.
type Usage struct {
	Month       string
	Block       string
	Rows        []Row
	Consumption energy.KWh
	Bought      energy.KWh
	Spend       money.Kobo
	Tokens      int
	Missing     int
}

// Service builds reports. Working out consumption reads every reading of
// every meter, so it is cached per month and dropped when readings change.
// Token sales are cheap and always summed fresh.
type Service struct {
	store *store.Store

	mu    sync.Mutex
	cache map[string]map[int]readings.MonthUsage // month -> meter id -> usage
}

// NewService returns a report service.
func NewService(s *store.Store) *Service {
	return &Service{store: s, cache: map[string]map[int]readings.MonthUsage{}}
}

// Invalidate forgets the cached consumption for a month.
func (s *Service) Invalidate(month string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, month)
}

func (s *Service) consumption(month string) map[int]readings.MonthUsage {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cache[month]; ok {
		return c
	}
	c := map[int]readings.MonthUsage{}
	for _, m := range s.store.Meters() {
		c[m.ID] = readings.Consumption(s.store.Readings(m.ID), month)
	}
	s.cache[month] = c
	return c
}

// Build returns the usage report for a month, optionally for one block ("" means all).
func (s *Service) Build(month, block string) Usage {
	u := Usage{Month: month, Block: block}
	start, err := clock.ParseMonth(month)
	if err != nil {
		return u
	}
	end := start.AddDate(0, 1, 0)
	cons := s.consumption(month)

	type sales struct {
		n      int
		units  energy.KWh
		amount money.Kobo
	}
	byMeter := map[int]sales{}
	for _, p := range s.store.Purchases() {
		if p.CreatedAt.Before(start) || !p.CreatedAt.Before(end) {
			continue
		}
		sl := byMeter[p.MeterID]
		sl.n++
		sl.units += p.Units
		sl.amount += p.Amount
		byMeter[p.MeterID] = sl
	}

	for _, f := range s.store.Flats() {
		if block != "" && f.Block != block {
			continue
		}
		m, err := s.store.Meter(f.MeterID)
		if err != nil {
			continue
		}
		sl := byMeter[m.ID]
		row := Row{Flat: f, Band: m.Band, Usage: cons[m.ID], Tokens: sl.n, Bought: sl.units, Spend: sl.amount}
		u.Rows = append(u.Rows, row)
		u.Tokens += sl.n
		u.Bought += sl.units
		u.Spend += sl.amount
		if row.Usage.Known {
			u.Consumption += row.Usage.Consumption
		} else {
			u.Missing++
		}
	}
	return u
}
