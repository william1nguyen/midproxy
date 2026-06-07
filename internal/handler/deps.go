package handler

import (
	"context"

	fhttp "github.com/bogdanfinn/fhttp"
)

type Fetcher interface {
	Fetch(ctx context.Context, url string, headers map[string]string,
		proxyURL string, cookies []*fhttp.Cookie) (string, int, error)
}

type ProxyPicker interface {
	Pick() string
	RecordSuccess(proxyURL string)
	RecordFailure(proxyURL string)
}

type CookieStore interface {
	Get(ctx context.Context, domain string) ([]*fhttp.Cookie, error)
	Set(ctx context.Context, domain string, cookies []*fhttp.Cookie) error
}

type Solver interface {
	Solve(ctx context.Context, targetURL, proxyURL string) ([]*fhttp.Cookie, error)
}

type CFDetector interface {
	IsChallenge(statusCode int, body string) bool
}

type Deps struct {
	Fetcher     Fetcher
	ProxyPicker ProxyPicker
	CookieStore CookieStore
	Solver      Solver
	CFDetector  CFDetector
}
