package fetcher_test

import (
	"testing"

	"github.com/william1nguyen/midproxy/internal/fetcher"
)

func TestIsCloudflareChallenge(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		body   string
		expect bool
	}{
		{"403 is always CF", 403, "<html>forbidden</html>", true},
		{"challenge text", 200, "<html>Just a moment...</html>", true},
		{"normal page", 200, "<html>Hello world</html>", false},
		{"cf_chl token", 200, "<html>_cf_chl_opt</html>", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fetcher.IsCloudflareChallenge(tt.code, tt.body)
			if got != tt.expect {
				t.Errorf("IsCloudflareChallenge(%d, %q) = %v, want %v", tt.code, tt.body, got, tt.expect)
			}
		})
	}
}
