// Package state holds dashboard-local read-state (panel preferences,
// inbox-item read flags). Lives on the host filesystem under
// ~/.personal-dashboard/state/ so the dashboard never writes back to
// the vault or to agentic-os-kai.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// InboxRead tracks which daily-* inbox items the user has marked read.
// Persisted as a flat JSON object on disk.
type InboxRead struct {
	mu   sync.Mutex
	path string
	read map[string]bool
}

// LoadInboxRead opens (or creates) the inbox-read.json file at the
// given path and returns the in-memory store. Missing file is fine.
func LoadInboxRead(path string) (*InboxRead, error) {
	r := &InboxRead{path: path, read: make(map[string]bool)}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return r, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return r, nil
	}
	if err := json.Unmarshal(data, &r.read); err != nil {
		// Corrupted state file should not crash the daemon. Start fresh,
		// the next write will overwrite the bad file.
		r.read = make(map[string]bool)
	}
	return r, nil
}

// IsRead reports whether the item id has been marked read.
func (r *InboxRead) IsRead(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.read[id]
}

// MarkRead flips the flag and persists. Persistence failures are returned
// to the caller so the handler can decide how to surface them.
func (r *InboxRead) MarkRead(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.read[id] = true
	return r.flushLocked()
}

// MarkUnread reverses MarkRead.
func (r *InboxRead) MarkUnread(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.read, id)
	return r.flushLocked()
}

func (r *InboxRead) flushLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(r.read)
	if err != nil {
		return err
	}
	// Write+rename for atomicity so a crash mid-write does not blank
	// the state.
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}
