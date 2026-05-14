package voice

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnabled(t *testing.T) {
	if (&Client{}).Enabled() {
		t.Fatal("zero-value client should be disabled")
	}
	if (&Client{APIKey: "x"}).Enabled() {
		t.Fatal("missing voice id should be disabled")
	}
	if (&Client{VoiceID: "x"}).Enabled() {
		t.Fatal("missing api key should be disabled")
	}
	if !New("x", "y").Enabled() {
		t.Fatal("New(x,y) should be enabled")
	}
}

func TestSynthesize_DisabledClientErrors(t *testing.T) {
	_, err := (&Client{}).Synthesize(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error from disabled client")
	}
}

func TestSynthesize_RequestShape(t *testing.T) {
	var seenAuth, seenAccept, seenCT string
	var seenBody string
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("xi-api-key")
		seenAccept = r.Header.Get("Accept")
		seenCT = r.Header.Get("Content-Type")
		seenPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		_, _ = w.Write([]byte("FAKE_MP3"))
	}))
	defer srv.Close()

	c := New("test-key", "voice-abc")
	c.httpClient = srv.Client()
	// Redirect the base by temporarily wrapping Do via a roundtripper would
	// be cleaner, but cheaper here is to point the test at srv.URL.
	c.httpClient.Transport = rewritingTransport{base: srv.URL, inner: http.DefaultTransport}

	audio, err := c.Synthesize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if string(audio) != "FAKE_MP3" {
		t.Fatalf("audio = %q, want FAKE_MP3", audio)
	}
	if seenAuth != "test-key" {
		t.Fatalf("xi-api-key = %q, want test-key", seenAuth)
	}
	if seenAccept != "audio/mpeg" {
		t.Fatalf("Accept = %q", seenAccept)
	}
	if seenCT != "application/json" {
		t.Fatalf("Content-Type = %q", seenCT)
	}
	if !strings.Contains(seenPath, "voice-abc") {
		t.Fatalf("path %q missing voice id", seenPath)
	}
	if !strings.Contains(seenBody, `"text":"hello"`) {
		t.Fatalf("body missing text field: %s", seenBody)
	}
}

// rewritingTransport replaces the ElevenLabs host with the test server's
// host so Synthesize hits httptest without changing the production URL.
type rewritingTransport struct {
	base  string
	inner http.RoundTripper
}

func (t rewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Force any outbound request through the test server, preserving path.
	clone := req.Clone(req.Context())
	srvURL := t.base
	// strip scheme prefix
	if i := strings.Index(srvURL, "://"); i >= 0 {
		srvURL = srvURL[i+3:]
	}
	clone.URL.Scheme = "http"
	clone.URL.Host = srvURL
	clone.Host = srvURL
	return t.inner.RoundTrip(clone)
}
