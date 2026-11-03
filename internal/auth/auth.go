// Package auth signs users in with a cookie session.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/palmgrove-estates/meterly/internal/store"
)

// CookieName is the session cookie.
const CookieName = "meterly_session"

const sessionTTL = 12 * time.Hour

type session struct {
	userID  int
	expires time.Time
}

// Sessions keeps signed-in users in memory.
type Sessions struct {
	store *store.Store

	mu       sync.Mutex
	sessions map[string]session
}

// NewSessions returns an empty session table.
func NewSessions(s *store.Store) *Sessions {
	return &Sessions{store: s, sessions: map[string]session{}}
}

// Authenticate checks an email and password.
func (a *Sessions) Authenticate(email, password string) (store.User, bool) {
	u, ok := a.store.UserByEmail(email)
	if !ok || subtle.ConstantTimeCompare([]byte(u.Password), []byte(password)) != 1 {
		return store.User{}, false
	}
	return u, true
}

// Start creates a session for u and sets the cookie.
func (a *Sessions) Start(w http.ResponseWriter, u store.User) {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	token := hex.EncodeToString(buf)
	a.mu.Lock()
	a.sessions[token] = session{userID: u.ID, expires: time.Now().Add(sessionTTL)}
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// End removes the session and clears the cookie.
func (a *Sessions) End(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1})
}

// Current returns the signed-in user, if any.
func (a *Sessions) Current(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return store.User{}, false
	}
	a.mu.Lock()
	s, ok := a.sessions[c.Value]
	a.mu.Unlock()
	if !ok || time.Now().After(s.expires) {
		return store.User{}, false
	}
	return a.store.User(s.userID)
}

type ctxKey struct{}

// WithUser stores the user on the context.
func WithUser(ctx context.Context, u store.User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// UserFrom reads the user stored by Require.
func UserFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(ctxKey{}).(store.User)
	return u, ok
}

// Require only lets signed-in users with one of the roles through. With no
// roles any signed-in user is allowed.
func (a *Sessions) Require(next http.HandlerFunc, roles ...store.Role) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := a.Current(r)
		if !ok {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}
		if len(roles) > 0 && !slices.Contains(roles, u.Role) {
			http.Error(w, "You do not have access to this page.", http.StatusForbidden)
			return
		}
		next(w, r.WithContext(WithUser(r.Context(), u)))
	}
}
