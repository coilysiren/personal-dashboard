// Package vaultinbox reads daily-* routine outputs from the Obsidian
// vault inbox. Files are named YYYY-MM-DD-<category>.md with a YAML
// frontmatter and a markdown body that often contains an
// llm-synthesis block (delimited by HTML comments) summarizing the
// day in that category.
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/43
package vaultinbox

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DailyFile is the parsed shape of one inbox markdown file.
type DailyFile struct {
	ID        string    // filename without extension: 2026-05-13-operational
	Date      string    // 2026-05-13
	Category  string    // operational, errors, social, etc.
	Title     string    // "Operational - 2026-05-13"
	Synthesis string    // body between llm-synthesis markers, or first paragraph
	ModTime   time.Time // file mtime, used for sort
}

// Source reads daily-* files from a directory.
type Source struct {
	Dir string
}

// New returns a Source pointed at the configured inbox path or a sane
// default. The vault is local to each host; on the Mac the default
// is the canonical coilyco-vault path. Override at construction.
func New(dir string) *Source {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "projects", "coilysiren", "coilyco-vault", "Obsidian Vault", "Inbox")
	}
	return &Source{Dir: dir}
}

// dailyFileRe matches YYYY-MM-DD-<category>.md. Files that don't match
// (e.g. one-off vault notes that share the inbox) are skipped.
var dailyFileRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-([a-z]+)\.md$`)

// List returns daily-* files sorted newest-first, capped at limit (0 =
// no limit). A missing inbox dir is treated as zero entries.
func (s *Source) List(limit int) ([]DailyFile, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var out []DailyFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := dailyFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		full := filepath.Join(s.Dir, e.Name())
		df, err := parseFile(full, m[1], m[2])
		if err != nil {
			// Skip malformed individual files; the panel still renders
			// everything else.
			continue
		}
		out = append(out, df)
	}

	sort.Slice(out, func(i, j int) bool {
		// Date desc, then category alpha to keep the order stable.
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Category < out[j].Category
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

var (
	synthStart = []byte("<!-- llm-synthesis:start -->")
	synthEnd   = []byte("<!-- llm-synthesis:end -->")
)

func parseFile(path, date, category string) (DailyFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return DailyFile{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return DailyFile{}, err
	}
	defer f.Close()

	df := DailyFile{
		ID:       strings.TrimSuffix(filepath.Base(path), ".md"),
		Date:     date,
		Category: category,
		ModTime:  info.ModTime(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inFrontmatter := false
	frontmatterDone := false
	inSynth := false
	var synth strings.Builder
	var firstPara strings.Builder
	firstParaDone := false

	for scanner.Scan() {
		line := scanner.Text()

		// Frontmatter handling.
		if !frontmatterDone {
			if !inFrontmatter && line == "---" {
				inFrontmatter = true
				continue
			}
			if inFrontmatter {
				if line == "---" {
					inFrontmatter = false
					frontmatterDone = true
					continue
				}
				continue
			}
			// Body started without frontmatter; fall through.
			frontmatterDone = true
		}

		// Pick up the H1 as the title if we haven't set one yet.
		if df.Title == "" && strings.HasPrefix(line, "# ") {
			df.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			continue
		}

		// Synthesis extraction.
		if !inSynth && strings.Contains(line, string(synthStart)) {
			inSynth = true
			continue
		}
		if inSynth {
			if strings.Contains(line, string(synthEnd)) {
				inSynth = false
				continue
			}
			synth.WriteString(line)
			synth.WriteByte('\n')
			continue
		}

		// First-paragraph fallback.
		if !firstParaDone && synth.Len() == 0 {
			trim := strings.TrimSpace(line)
			if trim == "" {
				if firstPara.Len() > 0 {
					firstParaDone = true
				}
				continue
			}
			if strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "<!--") {
				continue
			}
			if firstPara.Len() > 0 {
				firstPara.WriteByte(' ')
			}
			firstPara.WriteString(trim)
		}
	}
	if err := scanner.Err(); err != nil {
		return df, err
	}

	if df.Title == "" {
		df.Title = strings.Title(category) + " - " + date
	}
	df.Synthesis = strings.TrimSpace(synth.String())
	if df.Synthesis == "" {
		df.Synthesis = strings.TrimSpace(firstPara.String())
	}
	return df, nil
}
