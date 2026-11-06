// Package web serves Meterly's pages.
package web

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/palmgrove-estates/meterly/internal/auth"
	"github.com/palmgrove-estates/meterly/internal/billing"
	"github.com/palmgrove-estates/meterly/internal/clock"
	"github.com/palmgrove-estates/meterly/internal/meters"
	"github.com/palmgrove-estates/meterly/internal/prefs"
	"github.com/palmgrove-estates/meterly/internal/readings"
	"github.com/palmgrove-estates/meterly/internal/report"
	"github.com/palmgrove-estates/meterly/internal/store"
	"github.com/palmgrove-estates/meterly/ui"
)

// Server wires the services to HTTP handlers.
type Server struct {
	store    *store.Store
	auth     *auth.Sessions
	prefs    *prefs.Store
	meters   *meters.Service
	readings *readings.Service
	billing  *billing.Service
	reports  *report.Service
	pages    map[string]*template.Template
	log      *slog.Logger
}

// Deps are the services the server needs.
type Deps struct {
	Store  *store.Store
	Vendor meters.Vendor
	Log    *slog.Logger
}

// New builds a server and parses the templates.
func New(d Deps) (*Server, error) {
	s := &Server{
		store:    d.Store,
		auth:     auth.NewSessions(d.Store),
		prefs:    prefs.New(),
		meters:   meters.NewService(d.Store, d.Vendor),
		readings: readings.NewService(d.Store),
		billing:  billing.NewService(d.Store),
		reports:  report.NewService(d.Store),
		log:      d.Log,
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	s.readings.OnChange(s.reports.Invalidate)
	if err := s.parseTemplates(); err != nil {
		return nil, err
	}
	return s, nil
}

var funcs = template.FuncMap{
	"date":     func(t time.Time) string { return t.In(clock.Lagos).Format("02 Jan 2006") },
	"datetime": func(t time.Time) string { return t.In(clock.Lagos).Format("02 Jan 2006, 15:04") },
	"iso":      func(t time.Time) string { return t.In(clock.Lagos).Format(time.RFC3339) },
	"ymd":      func(t time.Time) string { return t.In(clock.Lagos).Format("2006-01-02") },
	"month":    clock.MonthLabel,
	"add":      func(a, b int) int { return a + b },
	"now":      clock.Today,
}

func (s *Server) parseTemplates() error {
	names, err := fs.Glob(ui.FS, "templates/*.html")
	if err != nil {
		return err
	}
	s.pages = map[string]*template.Template{}
	for _, name := range names {
		base := name[len("templates/"):]
		if base == "layout.html" {
			continue
		}
		t, err := template.New(base).Funcs(funcs).ParseFS(ui.FS, "templates/layout.html", name)
		if err != nil {
			return err
		}
		s.pages[base] = t
	}
	return nil
}

// Handler returns the routed handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	staff := []store.Role{store.RoleManager, store.RoleAccountant}
	req := s.auth.Require

	static, _ := fs.Sub(ui.FS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	mux.HandleFunc("GET /login", s.loginForm)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)

	mux.HandleFunc("GET /{$}", req(s.dashboard))
	mux.HandleFunc("GET /flats", req(s.flatList, staff...))
	mux.HandleFunc("GET /flats/{id}", req(s.flatDetail))
	mux.HandleFunc("POST /flats/{id}", req(s.updateResident, store.RoleManager))

	mux.HandleFunc("GET /meters/{id}", req(s.meterDetail))
	mux.HandleFunc("POST /meters/{id}/buy", req(s.buyUnits, store.RoleManager, store.RoleResident))
	mux.HandleFunc("GET /purchases/{id}", req(s.receipt))
	mux.HandleFunc("POST /purchases/{id}/refund", req(s.refundPurchase, store.RoleManager))
	mux.HandleFunc("POST /purchases/{id}/credit", req(s.creditPurchase, store.RoleManager))
	mux.HandleFunc("POST /meters/{id}/readings", req(s.addReading, store.RoleManager))
	mux.HandleFunc("POST /readings/{id}", req(s.correctReading, store.RoleManager))
	mux.HandleFunc("POST /readings/{id}/delete", req(s.removeReading, store.RoleManager))

	mux.HandleFunc("GET /tokens", req(s.tokenHistory, staff...))
	mux.HandleFunc("GET /charges", req(s.chargeList, staff...))
	mux.HandleFunc("GET /charges/{id}", req(s.invoiceDetail))
	mux.HandleFunc("POST /charges/{id}/payments", req(s.recordPayment, staff...))
	mux.HandleFunc("GET /reports/usage", req(s.usageReport, staff...))

	return s.logRequests(mux)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/healthz" {
			s.log.Debug("request", "method", r.Method, "path", r.URL.Path, "took", time.Since(start))
		}
	})
}

// render executes a page inside the layout. data may be nil.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page, title string, data map[string]any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "page not found: "+page, http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = map[string]any{}
	}
	data["Title"] = title
	if u, ok := auth.UserFrom(r.Context()); ok {
		data["User"] = u
		data["Staff"] = u.Role != store.RoleResident
		data["Manager"] = u.Role == store.RoleManager
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		s.log.Error("render", "page", page, "err", err)
		http.Error(w, "Something went wrong showing this page.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusNotFound, "notfound.html", "Not found", nil)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	s.log.Error("request failed", "path", r.URL.Path, "err", err)
	http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
}

func pathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil && id > 0
}

func currentUser(r *http.Request) store.User {
	u, _ := auth.UserFrom(r.Context())
	return u
}

// canSeeFlat lets staff see every flat and residents only their own.
func canSeeFlat(u store.User, flatID int) bool {
	return u.Role != store.RoleResident || u.FlatID == flatID
}
