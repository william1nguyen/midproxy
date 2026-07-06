package proxy

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/william1nguyen/midproxy/internal/fetch"
)

func TestRequestBypassesCache(t *testing.T) {
	tests := []struct {
		name   string
		method string
		header http.Header
		want   bool
	}{
		{name: "plain get", method: http.MethodGet, header: http.Header{}, want: false},
		{name: "non get", method: http.MethodPost, header: http.Header{}, want: true},
		{name: "authorization", method: http.MethodGet, header: http.Header{"Authorization": {"Bearer token"}}, want: true},
		{name: "cookie", method: http.MethodGet, header: http.Header{"Cookie": {"sid=1"}}, want: true},
		{name: "request no cache", method: http.MethodGet, header: http.Header{"Cache-Control": {"no-cache"}}, want: true},
		{name: "request no store", method: http.MethodGet, header: http.Header{"Cache-Control": {"max-age=60, no-store"}}, want: true},
		{name: "request zero max age", method: http.MethodGet, header: http.Header{"Cache-Control": {"max-age=0"}}, want: true},
		{name: "pragma no cache", method: http.MethodGet, header: http.Header{"Pragma": {"no-cache"}}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, "http://example.test/path", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header = tt.header
			if got := requestBypassesCache(req); got != tt.want {
				t.Fatalf("requestBypassesCache() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResponseCacheable(t *testing.T) {
	tests := []struct {
		name   string
		status int
		header http.Header
		want   bool
	}{
		{name: "plain ok", status: http.StatusOK, header: http.Header{}, want: true},
		{name: "non ok", status: http.StatusCreated, header: http.Header{}, want: false},
		{name: "set cookie", status: http.StatusOK, header: http.Header{"Set-Cookie": {"sid=1"}}, want: false},
		{name: "vary", status: http.StatusOK, header: http.Header{"Vary": {"Accept-Language"}}, want: false},
		{name: "private", status: http.StatusOK, header: http.Header{"Cache-Control": {"private"}}, want: false},
		{name: "no cache", status: http.StatusOK, header: http.Header{"Cache-Control": {"no-cache"}}, want: false},
		{name: "no store", status: http.StatusOK, header: http.Header{"Cache-Control": {"public, no-store"}}, want: false},
		{name: "zero max age", status: http.StatusOK, header: http.Header{"Cache-Control": {"max-age=0"}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &fetch.Response{StatusCode: tt.status, Header: tt.header}
			if got := responseCacheable(resp); got != tt.want {
				t.Fatalf("responseCacheable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheMaxAge(t *testing.T) {
	ttl, ok := cacheMaxAge(http.Header{"Cache-Control": {"public, max-age=60"}})
	if !ok || ttl.String() != "1m0s" {
		t.Fatalf("cacheMaxAge(max-age=60) = %v, %v, want 1m0s, true", ttl, ok)
	}

	ttl, ok = cacheMaxAge(http.Header{"Cache-Control": {"max-age=60", "s-maxage=10"}})
	if !ok || ttl.String() != "10s" {
		t.Fatalf("cacheMaxAge(s-maxage=10) = %v, %v, want 10s, true", ttl, ok)
	}

	if ttl, ok := cacheMaxAge(http.Header{"Cache-Control": {"max-age=0"}}); ok || ttl != 0 {
		t.Fatalf("cacheMaxAge(max-age=0) = %v, %v, want 0, false", ttl, ok)
	}
}

func TestPrepareBodyReplaySmallBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://example.test/post", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	req.GetBody = nil

	replayable, err := prepareBodyReplay(req)
	if err != nil {
		t.Fatal(err)
	}
	if !replayable {
		t.Fatal("expected small body to be replayable")
	}

	first, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	secondBody, err := req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	defer secondBody.Close()
	second, err := io.ReadAll(secondBody)
	if err != nil {
		t.Fatal(err)
	}

	if string(first) != "payload" || string(second) != "payload" {
		t.Fatalf("body replays = %q and %q, want payload", first, second)
	}
}

func TestPrepareBodyReplayLargeBodyKeepsSingleAttemptBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://example.test/post", strings.NewReader(strings.Repeat("x", maxReplayBodyBytes+2)))
	if err != nil {
		t.Fatal(err)
	}
	req.GetBody = nil

	replayable, err := prepareBodyReplay(req)
	if err != nil {
		t.Fatal(err)
	}
	if replayable {
		t.Fatal("expected large body to disable replay")
	}
	if req.GetBody != nil {
		t.Fatal("expected large body not to install GetBody")
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != maxReplayBodyBytes+2 {
		t.Fatalf("body length = %d, want %d", len(body), maxReplayBodyBytes+2)
	}
}
