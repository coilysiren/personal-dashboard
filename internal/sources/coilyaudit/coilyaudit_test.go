package coilyaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeJSONL(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDenials_FiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Unix()
	writeJSONL(t, filepath.Join(dir, "repo-a.jsonl"),
		`{"id":"a1","ts":`+itoa(now-300)+`,"decision":"accept","verb":"build","argv":["coily","exec","build"],"exit_code":0,"error":""}`,
		`{"id":"a2","ts":`+itoa(now-200)+`,"decision":"reject","verb":"gh.issue.create","argv":["coily","gh","issue","create","--title","(parens)"],"exit_code":1,"error":"policy: shell metacharacter rejected"}`,
		`{"id":"a3","ts":`+itoa(now-100)+`,"decision":"reject","verb":"gh.api","argv":["coily","gh","api","x?y|z"],"exit_code":1,"error":"policy: shell metacharacter rejected"}`,
	)
	writeJSONL(t, filepath.Join(dir, "repo-b.jsonl"),
		`{"id":"b1","ts":`+itoa(now-50)+`,"decision":"reject","verb":"docker","argv":["coily","docker","run"],"exit_code":1,"error":"verb not in allowlist"}`,
	)
	// Non-jsonl file is ignored.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := NewWithDir(dir).Denials(time.Time{}, 0)
	if err != nil {
		t.Fatalf("denials: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (a1 should be skipped: accept)", len(rows))
	}
	// Newest first.
	if rows[0].ID != "b1" {
		t.Fatalf("rows[0].ID = %s, want b1", rows[0].ID)
	}
	if rows[2].ID != "a2" {
		t.Fatalf("rows[2].ID = %s, want a2", rows[2].ID)
	}
}

func TestDenials_SinceCutoff(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.Add(-24 * time.Hour).Unix()
	recent := now.Add(-1 * time.Hour).Unix()
	writeJSONL(t, filepath.Join(dir, "log.jsonl"),
		`{"id":"old","ts":`+itoa(old)+`,"decision":"reject","verb":"x","argv":["x"],"exit_code":1,"error":"x"}`,
		`{"id":"new","ts":`+itoa(recent)+`,"decision":"reject","verb":"y","argv":["y"],"exit_code":1,"error":"y"}`,
	)
	rows, err := NewWithDir(dir).Denials(now.Add(-2*time.Hour), 0)
	if err != nil {
		t.Fatalf("denials: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "new" {
		t.Fatalf("rows = %+v, want only `new`", rows)
	}
}

func TestDenials_LimitCaps(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Unix()
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, `{"id":"r`+itoa(int64(i))+`","ts":`+itoa(now-int64(i))+`,"decision":"reject","verb":"v","argv":["v"],"exit_code":1,"error":"e"}`)
	}
	writeJSONL(t, filepath.Join(dir, "log.jsonl"), lines...)

	rows, err := NewWithDir(dir).Denials(time.Time{}, 10)
	if err != nil {
		t.Fatalf("denials: %v", err)
	}
	if len(rows) != 10 {
		t.Fatalf("rows = %d, want 10", len(rows))
	}
}

func TestDenials_MissingDirOK(t *testing.T) {
	rows, err := NewWithDir(filepath.Join(t.TempDir(), "no-such-dir")).Denials(time.Time{}, 0)
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if rows != nil {
		t.Fatalf("rows = %+v, want nil", rows)
	}
}

func TestDenials_MalformedLineSkipped(t *testing.T) {
	dir := t.TempDir()
	writeJSONL(t, filepath.Join(dir, "log.jsonl"),
		`{not-json`,
		`{"id":"good","ts":1,"decision":"reject","verb":"v","argv":["v"],"exit_code":1,"error":"e"}`,
	)
	rows, err := NewWithDir(dir).Denials(time.Time{}, 0)
	if err != nil {
		t.Fatalf("denials: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "good" {
		t.Fatalf("rows = %+v, want only `good`", rows)
	}
}

// itoa avoids strconv import in each line above; tiny helper.
func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
