package fetcher

import "strings"

var cfSignatures = []string{
	"Just a moment...",
	"challenge-platform",
	"cf-browser-verification",
	"Checking your browser",
	"_cf_chl",
}

func IsCloudflareChallenge(statusCode int, body string) bool {
	if statusCode == 403 {
		return true
	}
	for _, sig := range cfSignatures {
		if strings.Contains(body, sig) {
			return true
		}
	}
	return false
}
