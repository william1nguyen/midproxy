package fetcher

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsClient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/william1nguyen/midproxy/internal/proxy"
)

type Fetcher struct {
	timeout      time.Duration
	proxyManager *proxy.Manager
	mutex        sync.Mutex
	clients      map[string]tlsClient.HttpClient
}

func New(timeout time.Duration, proxyManager *proxy.Manager) *Fetcher {
	return &Fetcher{
		timeout:      timeout,
		proxyManager: proxyManager,
		clients:      make(map[string]tlsClient.HttpClient),
	}
}

func (f *Fetcher) getTLSClient(proxyURL string) (tlsClient.HttpClient, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if client, ok := f.clients[proxyURL]; ok {
		return client, nil
	}

	opts := []tlsClient.HttpClientOption{
		tlsClient.WithTimeoutSeconds(int(f.timeout.Seconds())),
		tlsClient.WithClientProfile(profiles.Chrome_120),
	}

	if proxyURL != "" {
		opts = append(opts, tlsClient.WithProxyUrl(proxyURL))
	}

	client, err := tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), opts...)
	if err != nil {
		return nil, err
	}

	f.clients[proxyURL] = client
	return client, nil
}

func (f *Fetcher) Fetch(ctx context.Context, url string, headers map[string]string, cookies []*fhttp.Cookie) (string, int, error) {
	if f.proxyManager == nil {
		return f.fetchWithProxy(ctx, url, headers, cookies, nil)
	}

	proxy := f.proxyManager.Pick()
	html, statusCode, err := f.fetchWithProxy(ctx, url, headers, cookies, proxy)
	if proxy != nil {
		if err == nil {
			f.proxyManager.RecordSuccess(proxy.URL)
		} else {
			f.proxyManager.RecordFailure(proxy.URL)
		}
	}
	return html, statusCode, err
}

func (f *Fetcher) fetchWithProxy(ctx context.Context, url string, headers map[string]string, cookies []*fhttp.Cookie, proxy *proxy.Proxy) (string, int, error) {
	proxyURL := ""
	if proxy != nil {
		proxyURL = proxy.URL
	}

	client, err := f.getTLSClient(proxyURL)
	if err != nil {
		return "", 0, fmt.Errorf("create tls client: %w", err)
	}

	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, url, nil)
	if err != nil {
		return "", 0, fmt.Errorf("create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	for _, c := range cookies {
		req.AddCookie(c)
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
