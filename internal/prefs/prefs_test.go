package prefs

import (
	"net/http/httptest"
	"testing"
)

func TestRememberKeepsLastChoice(t *testing.T) {
	p := New()
	got := p.Remember(httptest.NewRequest("GET", "/reports/usage?month=2026-09&block=B", nil), "month", "block")
	if got.Get("month") != "2026-09" || got.Get("block") != "B" {
		t.Fatalf("first visit: %v", got)
	}
	got = p.Remember(httptest.NewRequest("GET", "/reports/usage", nil), "month", "block")
	if got.Get("month") != "2026-09" || got.Get("block") != "B" {
		t.Fatalf("second visit: %v", got)
	}
	got = p.Remember(httptest.NewRequest("GET", "/reports/usage?block=all", nil), "month", "block")
	if got.Get("month") != "2026-09" || got.Get("block") != "all" {
		t.Fatalf("third visit: %v", got)
	}
}
