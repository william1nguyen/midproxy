package fetcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/william1nguyen/midproxy/internal/fetcher"
)

func TestFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	f := fetcher.New(5 * time.Second)
	html, code, err := f.Fetch(context.Background(), srv.URL, nil, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200, got %d", code)
	}
	if html != "<html>ok</html>" {
		t.Fatalf("unexpected html: %q", html)
	}
}
