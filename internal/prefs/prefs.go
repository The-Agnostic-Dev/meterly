// Package prefs remembers small screen choices, such as the month picked on
// a report, so a page opens the way it was left.
package prefs

import (
	"net/http"
	"net/url"
	"sync"
)

// Store keeps remembered choices in memory.
type Store struct {
	mu     sync.Mutex
	values map[string]url.Values
}

// New returns an empty store.
func New() *Store { return &Store{values: map[string]url.Values{}} }

// Remember merges the named query parameters with what was chosen last time
// on this page. Parameters present in the request win and are saved for
// next time; missing ones fall back to the saved value.
func (s *Store) Remember(r *http.Request, names ...string) url.Values {
	key := r.URL.Path
	query := r.URL.Query()

	s.mu.Lock()
	defer s.mu.Unlock()
	saved, ok := s.values[key]
	if !ok {
		saved = url.Values{}
		s.values[key] = saved
	}
	out := url.Values{}
	for _, name := range names {
		if query.Has(name) {
			saved.Set(name, query.Get(name))
		}
		if v := saved.Get(name); v != "" {
			out.Set(name, v)
		}
	}
	return out
}
