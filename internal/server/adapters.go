package server

import (
	"github.com/william1nguyen/midproxy/internal/fetcher"
	"github.com/william1nguyen/midproxy/internal/proxy"
)

type proxyPickerAdapter struct {
	proxyManager *proxy.Manager
}

func (a *proxyPickerAdapter) Pick() string {
	p := a.proxyManager.Pick()
	if p == nil {
		return ""
	}
	return p.URL
}

func (a *proxyPickerAdapter) RecordSuccess(proxyURL string) {
	a.proxyManager.RecordSuccess(proxyURL)
}

func (a *proxyPickerAdapter) RecordFailure(proxyURL string) {
	a.proxyManager.RecordFailure(proxyURL)
}

type cfDetectorAdapter struct{}

func (cfDetectorAdapter) IsChallenge(code int, body string) bool {
	return fetcher.IsCloudflareChallenge(code, body)
}
