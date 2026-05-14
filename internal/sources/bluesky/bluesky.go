// Package bluesky reads the user's recent posts via the public AppView.
//
// Unauthenticated read of the user's own profile feed. The issue scope
// (timeline + notifications) needs a real authenticated AT proto session
// with token refresh which is out of scope for the first ship. Profile
// feed gives the panel something real to show against any handle.
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/44
package bluesky

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Post is one feed item.
type Post struct {
	Text      string
	IndexedAt time.Time
	URI       string
}

// Client wraps the AT proto public AppView.
type Client struct {
	Handle     string
	httpClient *http.Client
}

func New(handle string) *Client {
	return &Client{
		Handle:     handle,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.Handle != "" }

type authorFeedResp struct {
	Feed []struct {
		Post struct {
			URI    string `json:"uri"`
			Record struct {
				Text      string `json:"text"`
				CreatedAt string `json:"createdAt"`
			} `json:"record"`
			IndexedAt time.Time `json:"indexedAt"`
		} `json:"post"`
	} `json:"feed"`
}

// Recent fetches the most recent posts from the configured handle.
func (c *Client) Recent(ctx context.Context, limit int) ([]Post, error) {
	if !c.Enabled() {
		return nil, errors.New("bluesky: client not configured")
	}
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("actor", c.Handle)
	q.Set("limit", fmt.Sprint(limit))
	endpoint := "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("bluesky: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bluesky: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("bluesky: %d: %s", resp.StatusCode, snippet)
	}

	var data authorFeedResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("bluesky: decode: %w", err)
	}
	out := make([]Post, 0, len(data.Feed))
	for _, f := range data.Feed {
		out = append(out, Post{
			Text:      f.Post.Record.Text,
			IndexedAt: f.Post.IndexedAt,
			URI:       f.Post.URI,
		})
	}
	return out, nil
}
