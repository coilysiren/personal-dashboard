// Package steam wraps the Steam Web API for the games widget.
//
// Auth: STEAM_API_KEY + STEAM_USER_ID env vars at daemon start. Missing
// either leaves the source disabled (panel renders an empty state
// rather than erroring).
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/46
package steam

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

// RecentlyPlayed is one game from the recently-played endpoint.
type RecentlyPlayed struct {
	AppID            int    `json:"appid"`
	Name             string `json:"name"`
	Playtime2Weeks   int    `json:"playtime_2weeks"`
	PlaytimeForever  int    `json:"playtime_forever"`
	ImgIconURL       string `json:"img_icon_url"`
}

// StoreURL is the Steam store page for a given app id.
func (r RecentlyPlayed) StoreURL() string {
	return fmt.Sprintf("https://store.steampowered.com/app/%d", r.AppID)
}

// IconURL is the small icon for the game; empty string if Steam didn't
// give us a hash.
func (r RecentlyPlayed) IconURL() string {
	if r.ImgIconURL == "" {
		return ""
	}
	return fmt.Sprintf("https://media.steampowered.com/steamcommunity/public/images/apps/%d/%s.jpg", r.AppID, r.ImgIconURL)
}

// Client wraps the Steam Web API.
type Client struct {
	APIKey string
	UserID string

	httpClient *http.Client
}

// New returns a Client. Missing apiKey or userID returns a Client where
// Enabled() reports false; the panel handler should render an empty
// state rather than crashing.
func New(apiKey, userID string) *Client {
	return &Client{
		APIKey: apiKey,
		UserID: userID,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Enabled reports whether the client has the credentials it needs.
func (c *Client) Enabled() bool {
	return c != nil && c.APIKey != "" && c.UserID != ""
}

type recentlyPlayedResponse struct {
	Response struct {
		TotalCount int              `json:"total_count"`
		Games      []RecentlyPlayed `json:"games"`
	} `json:"response"`
}

// Recent returns the user's recently-played games, capped at count.
func (c *Client) Recent(ctx context.Context, count int) ([]RecentlyPlayed, error) {
	if !c.Enabled() {
		return nil, errors.New("steam: client not configured")
	}
	if count <= 0 {
		count = 5
	}
	q := url.Values{}
	q.Set("key", c.APIKey)
	q.Set("steamid", c.UserID)
	q.Set("count", fmt.Sprint(count))

	endpoint := "https://api.steampowered.com/IPlayerService/GetRecentlyPlayedGames/v0001/?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("steam: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steam: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("steam: api returned %d: %s", resp.StatusCode, snippet)
	}

	var out recentlyPlayedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("steam: decode body: %w", err)
	}
	return out.Response.Games, nil
}
