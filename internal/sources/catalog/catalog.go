// Package catalog joins the static cross-repo dependency graph from
// coilyco-ai/data/catalog-graph.yaml with the live repo signals served
// by repo-recall, producing a single payload the catalog panel
// renders.
//
// The yaml is built by `coily exec build-catalog-graph` in coilyco-ai
// from each repo's `.coily/coily.yaml` catalog block. Repo-recall's
// dashboard JSON view supplies the live overlay (ci_status, open_issues,
// action_signals, deploy state, activity score). All GitHub data
// reaches this source through repo-recall, never via bare gh.
//
// Config:
//   - PERSONAL_DASHBOARD_COILYCO_AI_PATH: base dir of the coilyco-ai
//     checkout. The source reads data/catalog-graph.yaml underneath.
//     Default /home/kai/projects/coilysiren/coilyco-ai.
//   - REPO_RECALL_URL: base of the repo-recall HTTP API. Default
//     http://127.0.0.1:7777 (same host on kai-server).
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/58
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Source fronts the catalog graph plus repo-recall overlay.
type Source struct {
	CoilycoAIPath string
	RepoRecallURL string

	httpClient *http.Client
}

// New returns a Source. Empty fields fall back to the on-server
// defaults. Methods that need a path or URL return a degraded payload
// rather than erroring when the underlying file/endpoint is missing.
func New(coilycoAIPath, repoRecallURL string) *Source {
	if coilycoAIPath == "" {
		coilycoAIPath = "/home/kai/projects/coilysiren/coilyco-ai"
	}
	if repoRecallURL == "" {
		repoRecallURL = "http://127.0.0.1:7777"
	}
	return &Source{
		CoilycoAIPath: coilycoAIPath,
		RepoRecallURL: strings.TrimRight(repoRecallURL, "/"),
		httpClient:    &http.Client{Timeout: 5 * time.Second},
	}
}

// rawGraph mirrors the yaml shape produced by build-catalog-graph.
type rawGraph struct {
	Edges []Edge     `yaml:"edges"`
	Nodes []rawNode  `yaml:"nodes"`
}

type rawNode struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	System       string   `yaml:"system"`
	Type         string   `yaml:"type"`
	Kind         string   `yaml:"kind"`
	Lifecycle    string   `yaml:"lifecycle"`
	Owner        string   `yaml:"owner"`
	Description  string   `yaml:"description"`
	ProvidesApis []string `yaml:"providesApis"`
	ConsumesApis []string `yaml:"consumesApis"`
}

// Edge is a single dependsOn (or other) relation between two repos.
type Edge struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to"   json:"to"`
	Type string `yaml:"type" json:"type"`
}

// Node is a single repo in the graph, enriched with live signals when
// repo-recall has a matching row. Live is the zero-value when the
// repo isn't tracked by repo-recall (e.g. recently added, or scanned
// under a different path).
type Node struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	System       string   `json:"system"`
	Type         string   `json:"type"`
	Kind         string   `json:"kind"`
	Lifecycle    string   `json:"lifecycle"`
	Owner        string   `json:"owner"`
	Description  string   `json:"description"`
	ProvidesApis []string `json:"providesApis"`
	ConsumesApis []string `json:"consumesApis"`
	DependsOn    []string `json:"dependsOn"`
	DependedBy   []string `json:"dependedBy"`
	Live         Live     `json:"live"`
}

// Live mirrors the per-repo fields repo-recall exposes through its
// dashboard JSON view. Tags match repo-recall's wire shape so a future
// shared types crate can drop in. Zero-value indicates "repo-recall
// has no row for this repo."
type Live struct {
	HaveRow             bool   `json:"have_row"`
	CIStatus            string `json:"ci_status"`
	OpenIssues          int    `json:"open_issues"`
	OpenPRs             int    `json:"open_prs"`
	DraftPRs            int    `json:"draft_prs"`
	IssuesAssignedToMe  int    `json:"issues_assigned_to_me"`
	PRsAwaitingMyReview int    `json:"prs_awaiting_my_review"`
	ActionRequired      bool   `json:"action_required"`
	ActionSignals       []string `json:"action_signals"`
	Commits30d          int    `json:"commits_30d"`
	LocChurn30d         int    `json:"loc_churn_30d"`
	Authors30d          int    `json:"authors_30d"`
	ActivityScore       float64 `json:"activity_score"`
	DeployWorkflow      string `json:"deploy_workflow"`
	DeployStatus        string `json:"deploy_status"`
	DeployLastSuccessTs int64  `json:"deploy_last_success_ts"`
	HeadRef             string `json:"head_ref"`
	DefaultBranch       string `json:"default_branch"`
	CommitsAhead        int    `json:"commits_ahead"`
	CommitsBehind       int    `json:"commits_behind"`
	StashCount          int    `json:"stash_count"`
	ModifiedFiles       int    `json:"modified_files"`
	UntrackedFiles      int    `json:"untracked_files"`
	RemoteURL           string `json:"remote_url"`
	InProgressOp        string `json:"in_progress_op"`
}

// repoRecallRow is what repo-recall's `?format=json` returns inside
// "repos[]". Only the fields we surface are pulled.
type repoRecallRow struct {
	Name                string   `json:"name"`
	CIStatus            string   `json:"ci_status"`
	OpenIssues          int      `json:"open_issues"`
	OpenPRs             int      `json:"open_prs"`
	DraftPRs            int      `json:"draft_prs"`
	IssuesAssignedToMe  int      `json:"issues_assigned_to_me"`
	PRsAwaitingMyReview int      `json:"prs_awaiting_my_review"`
	ActionRequired      bool     `json:"action_required"`
	ActionSignals       []string `json:"action_signals"`
	Commits30d          int      `json:"commits_30d"`
	LocChurn30d         int      `json:"loc_churn_30d"`
	Authors30d          int      `json:"authors_30d"`
	ActivityScore       float64  `json:"activity_score"`
	DeployWorkflow      string   `json:"deploy_workflow"`
	DeployStatus        string   `json:"deploy_status"`
	DeployLastSuccessTs int64    `json:"deploy_last_success_ts"`
	HeadRef             string   `json:"head_ref"`
	DefaultBranch       string   `json:"default_branch"`
	CommitsAhead        int      `json:"commits_ahead"`
	CommitsBehind       int      `json:"commits_behind"`
	StashCount          int      `json:"stash_count"`
	ModifiedFiles       int      `json:"modified_files"`
	UntrackedFiles      int      `json:"untracked_files"`
	RemoteURL           string   `json:"remote_url"`
	InProgressOp        string   `json:"in_progress_op"`
}

// Catalog is the payload the panel renders against.
type Catalog struct {
	Nodes       []Node            `json:"nodes"`
	Edges       []Edge            `json:"edges"`
	Systems     []string          `json:"systems"`
	BySystem    map[string][]Node `json:"-"`
	GraphPath   string            `json:"-"`
	RepoRecall  string            `json:"-"`
	YamlErr     string            `json:"yaml_err"`
	RepoErr     string            `json:"repo_err"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// Fetch reads the yaml + repo-recall payload and joins them. Errors
// from either side land as YamlErr / RepoErr in the result so the
// panel can render a degraded view rather than a 500.
func (s *Source) Fetch(ctx context.Context) Catalog {
	c := Catalog{
		GraphPath:   s.CoilycoAIPath,
		RepoRecall:  s.RepoRecallURL,
		GeneratedAt: time.Now(),
		BySystem:    map[string][]Node{},
	}

	graphPath := filepath.Join(s.CoilycoAIPath, "data", "catalog-graph.yaml")
	raw, err := os.ReadFile(graphPath)
	if err != nil {
		c.YamlErr = err.Error()
		return c
	}
	var g rawGraph
	if err := yaml.Unmarshal(raw, &g); err != nil {
		c.YamlErr = fmt.Sprintf("parse %s: %v", graphPath, err)
		return c
	}

	// Forward + reverse adjacency from edges. Allocations are tiny;
	// 25 nodes, ~30 edges.
	depsOf := map[string][]string{}
	depBy := map[string][]string{}
	for _, e := range g.Edges {
		if e.Type != "dependsOn" {
			continue
		}
		depsOf[e.From] = append(depsOf[e.From], e.To)
		depBy[e.To] = append(depBy[e.To], e.From)
	}

	live, err := s.fetchLive(ctx)
	if err != nil {
		c.RepoErr = err.Error()
		// Fall through; graph still renders without overlay.
	}

	c.Nodes = make([]Node, 0, len(g.Nodes))
	systems := map[string]struct{}{}
	for _, rn := range g.Nodes {
		n := Node{
			ID:           rn.ID,
			Name:         rn.Name,
			System:       rn.System,
			Type:         rn.Type,
			Kind:         rn.Kind,
			Lifecycle:    rn.Lifecycle,
			Owner:        rn.Owner,
			Description:  rn.Description,
			ProvidesApis: rn.ProvidesApis,
			ConsumesApis: rn.ConsumesApis,
			DependsOn:    depsOf[rn.ID],
			DependedBy:   depBy[rn.ID],
		}
		sort.Strings(n.DependsOn)
		sort.Strings(n.DependedBy)
		if row, ok := live[rn.Name]; ok {
			n.Live = liveFromRow(row)
		}
		c.Nodes = append(c.Nodes, n)
		systems[rn.System] = struct{}{}
		c.BySystem[rn.System] = append(c.BySystem[rn.System], n)
	}
	c.Edges = g.Edges
	for sys := range systems {
		c.Systems = append(c.Systems, sys)
	}
	sort.Strings(c.Systems)
	for sys := range c.BySystem {
		sort.SliceStable(c.BySystem[sys], func(i, j int) bool {
			return c.BySystem[sys][i].Name < c.BySystem[sys][j].Name
		})
	}
	return c
}

func liveFromRow(r repoRecallRow) Live {
	return Live{
		HaveRow:             true,
		CIStatus:            r.CIStatus,
		OpenIssues:          r.OpenIssues,
		OpenPRs:             r.OpenPRs,
		DraftPRs:            r.DraftPRs,
		IssuesAssignedToMe:  r.IssuesAssignedToMe,
		PRsAwaitingMyReview: r.PRsAwaitingMyReview,
		ActionRequired:      r.ActionRequired,
		ActionSignals:       r.ActionSignals,
		Commits30d:          r.Commits30d,
		LocChurn30d:         r.LocChurn30d,
		Authors30d:          r.Authors30d,
		ActivityScore:       r.ActivityScore,
		DeployWorkflow:      r.DeployWorkflow,
		DeployStatus:        r.DeployStatus,
		DeployLastSuccessTs: r.DeployLastSuccessTs,
		HeadRef:             r.HeadRef,
		DefaultBranch:       r.DefaultBranch,
		CommitsAhead:        r.CommitsAhead,
		CommitsBehind:       r.CommitsBehind,
		StashCount:          r.StashCount,
		ModifiedFiles:       r.ModifiedFiles,
		UntrackedFiles:      r.UntrackedFiles,
		RemoteURL:           r.RemoteURL,
		InProgressOp:        r.InProgressOp,
	}
}

func (s *Source) fetchLive(ctx context.Context) (map[string]repoRecallRow, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.RepoRecallURL+"/?format=json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repo-recall: status %d", resp.StatusCode)
	}
	var payload struct {
		Repos []repoRecallRow `json:"repos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make(map[string]repoRecallRow, len(payload.Repos))
	for _, r := range payload.Repos {
		out[r.Name] = r
	}
	return out, nil
}
