package o2r

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchDigest_NoVM(t *testing.T) {
	d := New("http://g", "http://p", "").FetchDigest(context.Background())
	if d.HaveVM {
		t.Fatal("HaveVM should be false when URL is empty")
	}
	if d.SpanCount != 0 {
		t.Fatalf("SpanCount = %v, want 0", d.SpanCount)
	}
}

func TestFetchDigest_VMReturnsCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First candidate returns 0; second returns 42. Source should
		// keep trying until it finds a non-zero count.
		if q := r.URL.Query().Get("query"); q == spanMetricCandidates[0] {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[1234567890, "42.5"]}]}}`))
	}))
	defer srv.Close()

	d := New("g", "p", srv.URL).FetchDigest(context.Background())
	if !d.HaveVM {
		t.Fatal("HaveVM should be true")
	}
	if d.SpanCount != 42.5 {
		t.Fatalf("SpanCount = %v, want 42.5", d.SpanCount)
	}
}

func TestFetchDigest_VMUnreachable(t *testing.T) {
	d := New("g", "p", "http://127.0.0.1:1").FetchDigest(context.Background())
	if !d.HaveVM {
		t.Fatal("HaveVM should be true even when unreachable")
	}
	if d.Err == "" {
		t.Fatal("Err should be non-empty when VM unreachable")
	}
}
