package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/palmgrove-estates/meterly/internal/billing"
	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/money"
	"github.com/palmgrove-estates/meterly/internal/store"
)

var chargeStatuses = []string{"outstanding", billing.StatusUnpaid, billing.StatusPartPaid, billing.StatusOverdue, billing.StatusPaid}

func (s *Server) chargeList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	months := clock.RecentMonths(3)
	month := q.Get("month")
	if !slices.Contains(months, month) {
		month = months[0]
	}
	status := q.Get("status")
	if !slices.Contains(chargeStatuses, status) {
		status = ""
	}
	lines := s.billing.Lines(month, status)
	billed, received, outstanding := billing.Totals(lines)
	s.render(w, r, http.StatusOK, "charges.html", "Service charge", map[string]any{
		"Lines":       lines,
		"Months":      months,
		"Month":       month,
		"Statuses":    chargeStatuses,
		"Status":      status,
		"Billed":      billed,
		"Received":    received,
		"Outstanding": outstanding,
	})
}

func (s *Server) invoiceDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	line, err := s.billing.Line(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	u := currentUser(r)
	if !canSeeFlat(u, line.FlatID) {
		s.notFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, "invoice.html", "Service charge "+line.Flat.Label(), map[string]any{
		"Line":      line,
		"CanRecord": u.Role != store.RoleResident && line.Balance() > 0,
		"Today":     clock.Today(),
	})
}

type paymentResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Payment string `json:"payment,omitempty"`
	Paid    string `json:"paid"`
	Balance string `json:"balance"`
	Status  string `json:"status"`
}

func (s *Server) recordPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	amount, err := money.Parse(r.FormValue("amount"))
	receivedOn := clock.Today()
	if v := r.FormValue("received_on"); v != "" && err == nil {
		var d time.Time
		if d, err = clock.ParseDate(v); err == nil {
			receivedOn = d
		}
	}
	var inv store.Invoice
	if err == nil {
		inv, err = s.store.Invoice(id)
	}
	if err == nil && amount > inv.Balance() {
		err = fmt.Errorf("%s is more than the outstanding balance of %s", amount, inv.Balance())
	}
	if err == nil {
		inv, err = s.billing.RecordPayment(id, amount, receivedOn, currentUser(r).Name)
	} else if inv.ID == 0 {
		inv, _ = s.store.Invoice(id)
	}
	if inv.ID == 0 {
		s.notFound(w, r)
		return
	}
	resp := paymentResponse{
		OK:      err == nil,
		Paid:    inv.Paid().String(),
		Balance: inv.Balance().String(),
		Status:  billing.Status(inv, clock.Today()),
	}
	status := http.StatusOK
	if err != nil {
		resp.Error = err.Error()
		status = http.StatusUnprocessableEntity
	} else {
		resp.Payment = amount.String()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
