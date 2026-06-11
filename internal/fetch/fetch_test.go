package fetch_test

import (
	"testing"

	"github.com/william1nguyen/midproxy/internal/fetch"
)

func TestIsCloudflareChallenge_Normal200(t *testing.T) {
	result := fetch.IsCloudflareChallenge(200, []byte("<html><body>Normal page</body></html>"))
	if result != false {
		t.Errorf("expected false for status 200 with normal body, got true")
	}
}

func TestIsCloudflareChallenge_403WithMarker(t *testing.T) {
	result := fetch.IsCloudflareChallenge(403, []byte(`<html><body id="cf-browser-verification">Checking...</body></html>`))
	if result != true {
		t.Errorf("expected true for status 403 with cf-browser-verification, got false")
	}
}

func TestIsCloudflareChallenge_503WithJustAMoment(t *testing.T) {
	result := fetch.IsCloudflareChallenge(503, []byte("<html><title>Just a moment...</title></html>"))
	if result != true {
		t.Errorf("expected true for status 503 with 'Just a moment', got false")
	}
}

func TestIsCloudflareChallenge_403NoMarker(t *testing.T) {
	result := fetch.IsCloudflareChallenge(403, []byte("<html><body>Access denied</body></html>"))
	if result != false {
		t.Errorf("expected false for status 403 with no CF markers, got true")
	}
}

func TestIsCloudflareChallenge_200WithMarker(t *testing.T) {
	result := fetch.IsCloudflareChallenge(200, []byte(`<script>var cf_chl_opt = {};</script>`))
	if result != false {
		t.Errorf("expected false for status 200 even with cf_chl_opt marker, got true")
	}
}
