package session

import (
	"testing"
	"time"
)

func newTestStore(now func() time.Time, timeout time.Duration) *Store {
	s := NewStore()
	s.now = now
	s.timeout = timeout
	return s
}

func TestRevealHideRoundTrip(t *testing.T) {
	now := time.Now()
	s := newTestStore(func() time.Time { return now }, time.Minute)
	id := "session-a"

	if s.IsRevealed(id, "/") {
		t.Fatal("fresh session should be redacted")
	}
	s.Reveal(id, "/")
	if !s.IsRevealed(id, "/") {
		t.Fatal("after Reveal, IsRevealed should be true")
	}
	if s.IsRevealed(id, "/other") {
		t.Fatal("reveal is per-route, /other should still be redacted")
	}
	s.Hide(id, "/")
	if s.IsRevealed(id, "/") {
		t.Fatal("after Hide, IsRevealed should be false")
	}
}

func TestIdleExpiry(t *testing.T) {
	now := time.Now()
	s := newTestStore(func() time.Time { return now }, time.Minute)
	id := "session-b"

	s.Reveal(id, "/social")
	if !s.IsRevealed(id, "/social") {
		t.Fatal("fresh reveal should be visible")
	}

	now = now.Add(2 * time.Minute)
	if s.IsRevealed(id, "/social") {
		t.Fatal("after idle timeout, reveal should expire")
	}
}

func TestPruneEvictsStale(t *testing.T) {
	now := time.Now()
	s := newTestStore(func() time.Time { return now }, time.Minute)
	s.Reveal("a", "/")
	s.Reveal("b", "/")

	now = now.Add(2 * time.Minute)
	s.Reveal("c", "/") // fresh

	evicted := s.Prune()
	if evicted != 2 {
		t.Fatalf("evicted = %d, want 2", evicted)
	}
	if !s.IsRevealed("c", "/") {
		t.Fatal("fresh session c should survive prune")
	}
}

func TestNewIDUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if seen[id] {
			t.Fatalf("collision at iter %d: %s", i, id)
		}
		seen[id] = true
	}
}
