// Package o2r pulls a tiny digest from the otel-a2a-relay backend so
// the panel can show "most recent run" stats. The full waterfall and
// session list live in Grafana / Phoenix; this source just gives the
// dashboard enough to render an at-a-glance card with deep-links out.
//
// Config:
//   - GRAFANA_URL: base of the o2r Grafana (e.g. http://kai-server:3000).
//     The panel deep-links to /d/o2r-overview without auth handling
//     since the daemon and Grafana share the Tailscale network.
//   - PHOENIX_URL: base of the o2r Phoenix UI.
//   - VICTORIAMETRICS_URL: base of the span-metrics VM instance.
//     Optional. When set, the source queries a span count to populate
//     the digest line. When unset, the digest renders as "configure".
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/47
package o2r

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Source fronts the o2r backend.
type Source struct {
	GrafanaURL          string
	PhoenixURL          string
	VictoriaMetricsURL  string

	httpClient *http.Client
}

// New returns a Source. Empty fields are tolerated; methods that need
// a given URL return a disabled-state result instead of erroring.
func New(grafana, phoenix, vm string) *Source {
	return &Source{
		GrafanaURL:         grafana,
		PhoenixURL:         phoenix,
		VictoriaMetricsURL: vm,
		httpClient:         &http.Client{Timeout: 5 * time.Second},
	}
}

// Digest is the at-a-glance summary for the panel header.
type Digest struct {
	HaveVM    bool
	SpanCount float64
	Window    string
	Err       string
}

// Span totals over the configured window. Uses VM's prom-compatible
// /api/v1/query endpoint with a generic sum so the panel works
// regardless of which exact metric name the o2r pipeline emits, as
// long as one of the candidates is present.
//
// Candidate metrics are tried in order; the first one with a result
// wins. Add more here if o2r renames metrics.
var spanMetricCandidates = []string{
	`sum(rate(traces_spanmetrics_calls_total[1h]))`,
	`sum(rate(spans_total[1h]))`,
	`sum(rate(otelcol_processor_batch_batch_send_size_count[1h]))`,
}

// FetchDigest hits VictoriaMetrics for a span count over the last hour.
func (s *Source) FetchDigest(ctx context.Context) Digest {
	d := Digest{Window: "last 1h", HaveVM: s.VictoriaMetricsURL != ""}
	if !d.HaveVM {
		return d
	}
	for _, q := range spanMetricCandidates {
		v, err := s.queryVM(ctx, q)
		if err != nil {
			d.Err = err.Error()
			continue
		}
		if v > 0 {
			d.SpanCount = v
			d.Err = ""
			return d
		}
	}
	return d
}

type vmResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Value []any `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (s *Source) queryVM(ctx context.Context, query string) (float64, error) {
	endpoint := s.VictoriaMetricsURL + "/api/v1/query?query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("o2r: build request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("o2r: vm request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return 0, fmt.Errorf("o2r: vm returned %d: %s", resp.StatusCode, snippet)
	}
	var out vmResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("o2r: decode: %w", err)
	}
	if out.Status != "success" {
		return 0, errors.New("o2r: vm status != success")
	}
	if len(out.Data.Result) == 0 {
		return 0, nil
	}
	// Prom vector value is [timestamp_float, value_string].
	if len(out.Data.Result[0].Value) < 2 {
		return 0, errors.New("o2r: malformed vm result")
	}
	str, ok := out.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, errors.New("o2r: vm value not string")
	}
	return strconv.ParseFloat(str, 64)
}
