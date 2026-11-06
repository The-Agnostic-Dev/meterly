package web

import (
	"net/http"
	"slices"

	"github.com/palmgrove-estates/meterly/internal/clock"
)

var blocks = []string{"A", "B", "C"}

func sessionPreferenceScope(r *http.Request) string {
	return r.Header.Get("Cookie")
}

func (s *Server) usageReport(w http.ResponseWriter, r *http.Request) {
	q := s.prefs.Remember(r, sessionPreferenceScope(r), "month", "block")
	months := clock.RecentMonths(6)
	month := q.Get("month")
	if !slices.Contains(months, month) {
		month = months[0]
	}
	block := q.Get("block")
	if block != "all" && !slices.Contains(blocks, block) {
		block = "all"
	}
	filter := block
	if filter == "all" {
		filter = ""
	}
	usage := s.reports.Build(month, filter)
	s.render(w, r, http.StatusOK, "usage.html", "Estate usage", map[string]any{
		"Usage":  usage,
		"Month":  month,
		"Block":  block,
		"Months": months,
		"Blocks": blocks,
	})
}
