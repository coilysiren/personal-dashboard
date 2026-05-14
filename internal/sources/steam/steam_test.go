package steam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnabled(t *testing.T) {
	if (&Client{}).Enabled() {
		t.Fatal("zero client should be disabled")
	}
	if !New("k", "id").Enabled() {
		t.Fatal("New(k,id) should be enabled")
	}
}

func TestRecent_DisabledClientErrors(t *testing.T) {
	if _, err := (&Client{}).Recent(context.Background(), 5); err == nil {
		t.Fatal("expected error from disabled client")
	}
}

func TestRecent_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "k" {
			http.Error(w, "bad key", 400)
			return
		}
		if r.URL.Query().Get("steamid") != "id" {
			http.Error(w, "bad steamid", 400)
			return
		}
		_, _ = w.Write([]byte(`{"response":{"total_count":2,"games":[
			{"appid":12345,"name":"Test Game","playtime_2weeks":30,"playtime_forever":600,"img_icon_url":"abc"},
			{"appid":67890,"name":"Other","playtime_2weeks":15,"playtime_forever":200,"img_icon_url":""}
		]}}`))
	}))
	defer srv.Close()

	c := New("k", "id")
	c.httpClient = srv.Client()
	c.httpClient.Transport = rewritingTransport{base: srv.URL, inner: http.DefaultTransport}

	games, err := c.Recent(context.Background(), 5)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("len = %d, want 2", len(games))
	}
	if games[0].Name != "Test Game" {
		t.Fatalf("name = %q", games[0].Name)
	}
	if games[0].StoreURL() != "https://store.steampowered.com/app/12345" {
		t.Fatalf("store url = %q", games[0].StoreURL())
	}
	if games[1].IconURL() != "" {
		t.Fatalf("empty img hash should produce empty icon url, got %q", games[1].IconURL())
	}
}

type rewritingTransport struct {
	base  string
	inner http.RoundTripper
}

func (t rewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	host := t.base
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	clone.URL.Scheme = "http"
	clone.URL.Host = host
	clone.Host = host
	return t.inner.RoundTrip(clone)
}
