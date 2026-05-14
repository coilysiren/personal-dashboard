// Package session holds per-user reveal state for redact-by-default UI.
//
// Reveal model decided in #41:
//   - Per-page granularity. One reveal button per route. Each route starts
//     redacted until the user taps reveal.
//   - Per-session persistence. The session map remembers which routes
//     have been revealed. Idle timeout expires the whole session.
//
// Storage is an in-memory map. Daemon restart wipes everything, which is
// equivalent to closing the PWA. Acceptable for the threat model
// (shoulder-surfing in semi-public spaces).
package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// IdleTimeout is the default expiry for reveal state.
const IdleTimeout = 30 * time.Minute

// CookieName is the session cookie name.
const CookieName = "pd_session"

type entry struct {
	revealed   map[string]bool
	lastActive time.Time
}

// Store is the in-memory session map. Safe for concurrent use.
type Store struct {
	mu       sync.Mutex
	sessions map[string]*entry
	timeout  time.Duration
	now      func() time.Time
}

// NewStore returns a Store with the default idle timeout. Background
// pruning is the caller's responsibility (see StartPruner).
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*entry),
		timeout:  IdleTimeout,
		now:      time.Now,
	}
}

// NewID returns a random opaque session identifier.
func NewID() string {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("session: rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf[:])
}

// Touch records activity on the session, creating it if needed.
func (s *Store) Touch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.sessions[id]
	if !ok {
		e = &entry{revealed: make(map[string]bool)}
		s.sessions[id] = e
	}
	e.lastActive = s.now()
}

// IsRevealed reports whether the given route is revealed for the session.
// Stale sessions return false (and are evicted as a side effect).
func (s *Store) IsRevealed(id, route string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.sessions[id]
	if !ok {
		return false
	}
	if s.now().Sub(e.lastActive) > s.timeout {
		delete(s.sessions, id)
		return false
	}
	return e.revealed[route]
}

// Reveal marks a route revealed and touches the session.
func (s *Store) Reveal(id, route string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.sessions[id]
	if !ok {
		e = &entry{revealed: make(map[string]bool)}
		s.sessions[id] = e
	}
	e.revealed[route] = true
	e.lastActive = s.now()
}

// Hide marks a route redacted again.
func (s *Store) Hide(id, route string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.sessions[id]
	if !ok {
		return
	}
	delete(e.revealed, route)
	e.lastActive = s.now()
}

// Prune evicts sessions whose lastActive is older than the timeout.
// Returns the number of evicted entries.
func (s *Store) Prune() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.now().Add(-s.timeout)
	n := 0
	for id, e := range s.sessions {
		if e.lastActive.Before(cutoff) {
			delete(s.sessions, id)
			n++
		}
	}
	return n
}
