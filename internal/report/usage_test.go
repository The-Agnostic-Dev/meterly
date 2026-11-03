package report

import (
	"testing"

	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/energy"
	"github.com/palmgrove-estates/meterly/internal/readings"
	"github.com/palmgrove-estates/meterly/internal/store"
)

func TestBuildTotalsAndBlockFilter(t *testing.T) {
	st := store.New()
	st.Load(nil, []store.Band{{Code: "A", Rate: 20950}},
		[]store.Flat{{ID: 1, Block: "A", Number: 1, MeterID: 10}, {ID: 2, Block: "B", Number: 1, MeterID: 20}},
		[]store.Meter{{ID: 10, FlatID: 1, Band: "A"}, {ID: 20, FlatID: 2, Band: "A"}})
	rs := readings.NewService(st)
	svc := NewService(st)
	rs.OnChange(svc.Invalidate)

	month := clock.MonthKey(clock.Now())
	before := clock.StartOfMonth(clock.Now()).AddDate(0, 0, -3)
	for _, id := range []int{10, 20} {
		if _, err := rs.Add(id, before, energy.KWh(100000), "t"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := rs.Add(10, clock.Today(), energy.KWh(103000), "t"); err != nil {
		t.Fatal(err)
	}

	u := svc.Build(month, "")
	if len(u.Rows) != 2 || u.Consumption != 3000 || u.Missing != 1 {
		t.Fatalf("all blocks: rows=%d consumption=%v missing=%d", len(u.Rows), u.Consumption, u.Missing)
	}
	if _, err := rs.Add(20, clock.Today(), energy.KWh(100500), "t"); err != nil {
		t.Fatal(err)
	}
	if u := svc.Build(month, ""); u.Consumption != 3500 || u.Missing != 0 {
		t.Fatalf("after new reading: %v missing %d", u.Consumption, u.Missing)
	}
	if b := svc.Build(month, "B"); len(b.Rows) != 1 || b.Consumption != 500 {
		t.Fatalf("block B: %+v", b)
	}
}
