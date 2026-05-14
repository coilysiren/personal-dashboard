// Package reddit reads the user's unread message RSS feed.
//
// Auth: Reddit issues per-user private RSS URLs that embed an opaque
// token. The full URL is the credential. Configure it via REDDIT_INBOX_RSS
// (SecureString in SSM, materialized into the daemon env at start).
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/44
package reddit

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Item is one entry from the inbox.
type Item struct {
	Title   string
	Author  string
	Updated time.Time
	Link    string
}

// Client fetches a Reddit RSS inbox.
type Client struct {
	URL        string
	httpClient *http.Client
}

func New(rssURL string) *Client {
	return &Client{
		URL:        rssURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.URL != "" }

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string    `xml:"title"`
	Author  atomName  `xml:"author"`
	Updated time.Time `xml:"updated"`
	Link    atomLink  `xml:"link"`
}

type atomName struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
}

// Unread fetches the configured RSS URL and returns parsed items.
func (c *Client) Unread(ctx context.Context) ([]Item, error) {
	if !c.Enabled() {
		return nil, errors.New("reddit: client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("reddit: build request: %w", err)
	}
	// Reddit gates non-browser UAs harshly; a clear identifier is required.
	req.Header.Set("User-Agent", "personal-dashboard/1.0 (by /u/coilysiren)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("reddit: %d: %s", resp.StatusCode, snippet)
	}

	var feed atomFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("reddit: decode: %w", err)
	}
	out := make([]Item, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		out = append(out, Item{
			Title:   e.Title,
			Author:  e.Author.Name,
			Updated: e.Updated,
			Link:    e.Link.Href,
		})
	}
	return out, nil
}
