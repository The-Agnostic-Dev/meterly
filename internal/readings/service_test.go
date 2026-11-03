package readings

import (
	"testing"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/store"
)

func setup(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st := store.New()
	st.Load(nil, nil, []store.Flat{{ID: 1, Block: "A", Number: 1, MeterID: 10}}, []store.Meter{{ID: 10, FlatID: 1, Band: "A"}})
	return NewService(st), st
}

func TestConsumptionUsesReadingBeforeMonth(t *testing.T) {
	sep, _ := clock.ParseDate("2026-09-28")
	oct, _ := clock.ParseDate("2026-10-28")
	octEarly, _ := clock.ParseDate("2026-10-05")
	history := []store.Reading{
		{Date: sep, Value: 100000},
		{Date: octEarly, Value: 103000},
		{Date: oct, Value: 109550},
	}
	u := Consumption(history, "2026-10")
	if !u.Known || u.Consumption != 9550 || u.Opening != 100000 || u.Closing != 109550 {
		t.Fatalf("got %+v", u)
	}
	if Consumption(history, "2026-09").Known {
		t.Fatal("September has no opening reading")
	}
}

func TestAddAndNotify(t *testing.T) {
	svc, st := setup(t)
	var months []string
	svc.OnChange(func(m string) { months = append(months, m) })
	day := clock.Today().AddDate(0, 0, -3)
	if _, err := svc.Add(10, day, energy.KWh(500000), "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(10, clock.Today(), energy.KWh(501250), "test"); err != nil {
		t.Fatal(err)
	}
	if len(st.Readings(10)) != 2 || len(months) != 2 {
		t.Fatalf("readings=%d notifications=%d", len(st.Readings(10)), len(months))
	}
}

func TestAddRefusesFutureDate(t *testing.T) {
	svc, _ := setup(t)
	if _, err := svc.Add(10, clock.Today().AddDate(0, 0, 1), energy.KWh(100), "test"); err != ErrFuture {
		t.Fatalf("err = %v", err)
	}
}

func TestCorrectStaysBetweenNeighbours(t *testing.T) {
	svc, _ := setup(t)
	a, _ := svc.Add(10, clock.Today().AddDate(0, 0, -20), energy.KWh(100000), "t")
	b, _ := svc.Add(10, clock.Today().AddDate(0, 0, -10), energy.KWh(105000), "t")
	_, _ = svc.Add(10, clock.Today(), energy.KWh(110000), "t")
	if _, err := svc.Correct(b.ID, energy.KWh(111000), "t"); err == nil {
		t.Fatal("a correction above the next reading should be refused")
	}
	if _, err := svc.Correct(b.ID, energy.KWh(99000), "t"); err == nil {
		t.Fatal("a correction below the previous reading should be refused")
	}
	got, err := svc.Correct(b.ID, energy.KWh(104000), "t")
	if err != nil || got.Value != 104000 {
		t.Fatalf("correct: %v %v", got.Value, err)
	}
	if _, err := svc.Correct(a.ID, energy.KWh(90000), "t"); err != nil {
		t.Fatalf("first reading can go lower: %v", err)
	}
}
