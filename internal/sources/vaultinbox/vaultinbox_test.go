package vaultinbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const sampleFile = `---
date: '2026-05-13'
category: operational
status: complete
---

# Operational - 2026-05-13

## Synthesis

<!-- llm-synthesis:start -->
All green. Public endpoints up. K3s has four pods pending.
<!-- llm-synthesis:end -->

## Details

Body continues here.
`

func TestList_ParsesAndSorts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "2026-05-11-social.md"), sampleFile)
	writeFile(t, filepath.Join(dir, "2026-05-13-operational.md"), sampleFile)
	writeFile(t, filepath.Join(dir, "2026-05-12-errors.md"), sampleFile)
	// Non-daily file is ignored.
	writeFile(t, filepath.Join(dir, "random-note.md"), "# stray")

	out, err := New(dir).List(0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("got %d entries, want 3", len(out))
	}
	if out[0].ID != "2026-05-13-operational" {
		t.Fatalf("newest = %s, want 2026-05-13-operational", out[0].ID)
	}
	if out[2].ID != "2026-05-11-social" {
		t.Fatalf("oldest = %s, want 2026-05-11-social", out[2].ID)
	}
}

func TestParseFile_SynthesisExtracted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-05-13-operational.md")
	writeFile(t, path, sampleFile)

	out, _ := New(dir).List(0)
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	df := out[0]
	if df.Title != "Operational - 2026-05-13" {
		t.Fatalf("title = %q", df.Title)
	}
	if !strings.Contains(df.Synthesis, "All green") {
		t.Fatalf("synthesis missing expected text: %q", df.Synthesis)
	}
	if df.Date != "2026-05-13" || df.Category != "operational" {
		t.Fatalf("date/category wrong: %s / %s", df.Date, df.Category)
	}
}

func TestParseFile_FirstParaFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-05-13-memory.md")
	writeFile(t, path, `---
date: '2026-05-13'
category: memory
---

# Memory - 2026-05-13

This is the first paragraph.

A second one nobody reads.
`)
	out, _ := New(dir).List(0)
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0].Synthesis != "This is the first paragraph." {
		t.Fatalf("synthesis = %q", out[0].Synthesis)
	}
}

func TestList_Limit(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 5; i++ {
		date := "2026-05-0" + string(rune('0'+i))
		writeFile(t, filepath.Join(dir, date+"-operational.md"), sampleFile)
	}
	out, _ := New(dir).List(3)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
}

func TestList_MissingDirOK(t *testing.T) {
	out, err := New(filepath.Join(t.TempDir(), "nope")).List(0)
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if out != nil {
		t.Fatalf("got %+v, want nil", out)
	}
}
