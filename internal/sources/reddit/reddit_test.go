package reddit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>comment reply: thread title</title>
    <author><name>/u/someone</name></author>
    <updated>2026-05-13T10:00:00+00:00</updated>
    <link href="https://reddit.com/r/golang/comments/abc"/>
  </entry>
  <entry>
    <title>message: hi</title>
    <author><name>/u/anotherone</name></author>
    <updated>2026-05-12T08:00:00+00:00</updated>
    <link href="https://reddit.com/message/messages/xyz"/>
  </entry>
</feed>
`

func TestEnabled(t *testing.T) {
	if (&Client{}).Enabled() {
		t.Fatal("zero client should be disabled")
	}
	if !New("https://reddit.example/feed.rss").Enabled() {
		t.Fatal("New with URL should be enabled")
	}
}

func TestUnread_Parses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("User-Agent"), "personal-dashboard") {
			http.Error(w, "missing UA", 400)
			return
		}
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	items, err := New(srv.URL).Unread(context.Background())
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0].Title != "comment reply: thread title" {
		t.Fatalf("title = %q", items[0].Title)
	}
	if items[0].Author != "/u/someone" {
		t.Fatalf("author = %q", items[0].Author)
	}
}

func TestUnread_DisabledErrors(t *testing.T) {
	if _, err := (&Client{}).Unread(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
