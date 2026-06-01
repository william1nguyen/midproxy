package fetcher

import (
	"context"
	"fmt"
	"io"

	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/william1nguyen/midproxy/internal/proxy"
)

type Fetcher struct {
	timeout      time.Duration
	proxyManager *proxy.Manager
}

func New(timeout time.Duration, proxyManager *proxy.Manager) *Fetcher {
	return &Fetcher{
		timeout:      timeout,
		proxyManager: proxyManager,
	}
}

func (fetcher *Fetcher) Fetch(ctx context.Context, url string, headers map[string]string, cookies []*fhttp.Cookie) (string, int, error) {
	if fetcher.proxyManager == nil {
		return fetcher.fetchWithProxy(ctx, url, headers, cookies, nil)
	}

	proxy := fetcher.proxyManager.Pick()
	html, statusCode, err := fetcher.fetchWithProxy(ctx, url, headers, cookies, proxy)
	if proxy != nil {
		if err == nil {
			fetcher.proxyManager.RecordSuccess(proxy.URL)
		} else {
			fetcher.proxyManager.RecordFailure(proxy.URL)
		}
	}
	return html, statusCode, err
}

func (fetcher *Fetcher) fetchWithProxy(ctx context.Context, url string, headers map[string]string, cookies []*fhttp.Cookie, proxy *proxy.Proxy) (string, int, error) {
	opts := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(int(fetcher.timeout.Seconds())),
		tls_client.WithClientProfile(profiles.Chrome_120),
	}

	if proxy != nil {
		opts = append(opts, tls_client.WithProxyUrl(proxy.URL))
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		return "", 0, fmt.Errorf("create tls client: %w", err)
	}

	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, url, nil)
	if err != nil {
		return "", 0, fmt.Errorf("create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	response, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("do request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read request: %w", err)
	}

	return string(body), response.StatusCode, nil
}
