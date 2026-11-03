package web

import (
	"net/http"
	"strings"
)

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth.Current(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "login.html", "Sign in", map[string]any{"Next": r.URL.Query().Get("next")})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	next := r.FormValue("next")
	u, ok := s.auth.Authenticate(email, r.FormValue("password"))
	if !ok {
		s.render(w, r, http.StatusUnauthorized, "login.html", "Sign in", map[string]any{
			"Email": email, "Next": next, "Error": "That email and password do not match.",
		})
		return
	}
	s.auth.Start(w, u)
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.auth.End(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
