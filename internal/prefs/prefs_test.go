package prefs

import (
	"net/http/httptest"
	"testing"
)

func TestRememberKeepsLastChoicePerScope(t *testing.T) {
	p := New()

	got := p.Remember(httptest.NewRequest("GET", "/reports/usage?month=2026-09&block=B", nil), "session-a", "month", "block")
	if got.Get("month") != "2026-09" || got.Get("block") != "B" {
		t.Fatalf("first visit: %v", got)
	}
	got = p.Remember(httptest.NewRequest("GET", "/reports/usage", nil), "session-a", "month", "block")
	if got.Get("month") != "2026-09" || got.Get("block") != "B" {
		t.Fatalf("second visit: %v", got)
	}

	got = p.Remember(httptest.NewRequest("GET", "/reports/usage", nil), "session-b", "month", "block")
	if got.Get("month") != "" || got.Get("block") != "" {
		t.Fatalf("new scope inherited choices: %v", got)
	}
	got = p.Remember(httptest.NewRequest("GET", "/reports/usage?month=2026-08&block=all", nil), "session-b", "month", "block")
	if got.Get("month") != "2026-08" || got.Get("block") != "all" {
		t.Fatalf("other scope first visit: %v", got)
	}
	got = p.Remember(httptest.NewRequest("GET", "/reports/usage", nil), "session-b", "month", "block")
	if got.Get("month") != "2026-08" || got.Get("block") != "all" {
		t.Fatalf("other scope second visit: %v", got)
	}

	got = p.Remember(httptest.NewRequest("GET", "/reports/usage", nil), "session-a", "month", "block")
	if got.Get("month") != "2026-09" || got.Get("block") != "B" {
		t.Fatalf("first scope changed by other scope: %v", got)
	}
}
