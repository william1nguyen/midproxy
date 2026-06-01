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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := fetcher.IsCloudflareChallenge(test.code, test.body)
			if got != test.expect {
				t.Errorf("IsCloudflareChallenge(%d, %q) = %v, want %v", test.code, test.body, got, test.expect)
			}
		})
	}
}
