// Package coilyaudit reads coily's per-repo JSONL audit logs and surfaces
// denials. Denials are rows with decision == "reject"; these are the
// commands coily refused to run because of the lockdown policy. Each one
// is a candidate for adding to .coily/coily.yaml or relaxing a rule.
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/48
package coilyaudit

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultDir is the standard location of the coily audit log on macOS
// and Linux. Override at construction time via NewWithDir.
const DefaultDir = ".coily/audit"

// Row is the subset of an audit entry the panel needs. The on-disk
// schema is richer (egress, repo_root, commit_scope, duration_ms);
// fields not used here are intentionally dropped.
type Row struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"-"`
	Decision string    `json:"decision"`
	Verb     string    `json:"verb"`
	Argv     []string  `json:"argv"`
	ExitCode int       `json:"exit_code"`
	Error    string    `json:"error"`
}

// rawRow mirrors the disk shape so we can pull the unix timestamp into
// a real time.Time without exporting an int field.
type rawRow struct {
	ID       string   `json:"id"`
	TS       int64    `json:"ts"`
	Decision string   `json:"decision"`
	Verb     string   `json:"verb"`
	Argv     []string `json:"argv"`
	ExitCode int      `json:"exit_code"`
	Error    string   `json:"error"`
}

// Source reads denials from the coily audit log.
type Source struct {
	Dir string
}

// New points at ~/.coily/audit (the conventional location).
func New() *Source {
	home, _ := os.UserHomeDir()
	return &Source{Dir: filepath.Join(home, DefaultDir)}
}

// NewWithDir lets callers (tests, kai-server with a custom layout)
// override the directory.
func NewWithDir(dir string) *Source { return &Source{Dir: dir} }

// Denials returns reject rows newer than `since`, most recent first,
// capped at `limit`. A missing audit directory is treated as zero
// denials rather than an error; the panel renders empty in that case.
func (s *Source) Denials(since time.Time, limit int) ([]Row, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var out []Row
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		rows, err := readDenialsFrom(filepath.Join(s.Dir, e.Name()), since)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Time.After(out[j].Time)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func readDenialsFrom(path string, since time.Time) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rows []Row
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw rawRow
		if err := json.Unmarshal(line, &raw); err != nil {
			// Skip malformed lines rather than failing the whole panel.
			continue
		}
		if raw.Decision != "reject" {
			continue
		}
		t := time.Unix(raw.TS, 0)
		if !since.IsZero() && t.Before(since) {
			continue
		}
		rows = append(rows, Row{
			ID:       raw.ID,
			Time:     t,
			Decision: raw.Decision,
			Verb:     raw.Verb,
			Argv:     raw.Argv,
			ExitCode: raw.ExitCode,
			Error:    raw.Error,
		})
	}
	if err := scanner.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}
