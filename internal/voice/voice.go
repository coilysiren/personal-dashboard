// Package voice wraps ElevenLabs TTS for event and phase announcements.
//
// Output side only. Voice is fire-and-forget: panels call Announce with
// a short string, the daemon synthesizes audio, the PWA plays it.
//
// Auth: the API key lives in SSM at /coilysiren/elevenlabs/api-key and
// is materialized into the daemon environment as ELEVENLABS_API_KEY at
// service start. Same shape for ELEVENLABS_VOICE_ID. Never commit either.
//
// Tracked: https://github.com/coilysiren/personal-dashboard/issues/42
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultModel is ElevenLabs's general-purpose model. Cheap, fast,
// quality is fine for short event announcements.
const DefaultModel = "eleven_flash_v2_5"

// APIBase is the ElevenLabs TTS endpoint root.
const APIBase = "https://api.elevenlabs.io/v1/text-to-speech"

// Client synthesizes audio from text.
type Client struct {
	APIKey  string
	VoiceID string
	Model   string

	httpClient *http.Client
}

// New returns a Client. Missing apiKey or voiceID returns an unusable
// Client (Synthesize errors out); the daemon should log a warning at
// startup and treat voice as disabled instead of crashing.
func New(apiKey, voiceID string) *Client {
	return &Client{
		APIKey:  apiKey,
		VoiceID: voiceID,
		Model:   DefaultModel,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Enabled reports whether the client has the credentials it needs.
func (c *Client) Enabled() bool {
	return c != nil && c.APIKey != "" && c.VoiceID != ""
}

// Synthesize returns mp3 bytes for the given text. Caller is responsible
// for streaming to a response or caching.
func (c *Client) Synthesize(ctx context.Context, text string) ([]byte, error) {
	if !c.Enabled() {
		return nil, errors.New("voice: client not configured (missing API key or voice id)")
	}
	if text == "" {
		return nil, errors.New("voice: empty text")
	}

	body, err := json.Marshal(map[string]any{
		"text":     text,
		"model_id": c.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("voice: marshal request: %w", err)
	}

	url := APIBase + "/" + c.VoiceID
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("voice: build request: %w", err)
	}
	req.Header.Set("xi-api-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voice: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("voice: api returned %d: %s", resp.StatusCode, snippet)
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("voice: read body: %w", err)
	}
	return audio, nil
}
