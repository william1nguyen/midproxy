package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsClient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/william1nguyen/midproxy/internal/cache"
	"github.com/william1nguyen/midproxy/internal/cookies"
	"github.com/william1nguyen/midproxy/internal/proxy"
	"github.com/william1nguyen/midproxy/internal/ratelimit"
)

type Fetcher struct {
	timeout      time.Duration
	mutex        sync.RWMutex
	clients      map[string]tlsClient.HttpClient
	proxyManager *proxy.Manager
	cache        *cache.Cache
	cookieStore  *cookies.Store
	limiter      *ratelimit.Limiter
}

func New(timeout time.Duration, proxyManager *proxy.Manager, cache *cache.Cache, cookieStore *cookies.Store, limiter *ratelimit.Limiter) *Fetcher {
	return &Fetcher{
		timeout:      timeout,
		proxyManager: proxyManager,
		clients:      make(map[string]tlsClient.HttpClient),
		cache:        cache,
		cookieStore:  cookieStore,
		limiter:      limiter,
	}
}

func (f *Fetcher) getTLSClient(proxyURL string) (tlsClient.HttpClient, error) {
	f.mutex.RLock()
	client, ok := f.clients[proxyURL]
	f.mutex.RUnlock()

	if ok {
		return client, nil
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()

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

func (f *Fetcher) Fetch(ctx context.Context, url string, headers map[string]string) (string, int, error) {
	domain := f.getDomainFromURL(url)

	if f.limiter != nil {
		if allowed, err := f.limiter.Allow(ctx, domain); err == nil && !allowed {
			return "", fhttp.StatusTooManyRequests, fmt.Errorf("rate limited: domain %s", domain)
		}
	}

	if f.cache != nil {
		if entry, err := f.cache.Get(ctx, url); err == nil && entry != nil {
			return entry.HTML, entry.StatusCode, nil
		}
	}

	var cookies []*fhttp.Cookie
	if f.cookieStore != nil {
		if c, err := f.cookieStore.Get(ctx, domain); err == nil {
			cookies = c
		}
	}

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

	requestSucceed := statusCode >= 200 && statusCode < 400
	if f.cache != nil && requestSucceed {
		f.cache.Set(ctx, url, &cache.Entry{HTML: html, StatusCode: statusCode})
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
		return "", fhttp.StatusBadGateway, fmt.Errorf("create tls client: %w", err)
	}

	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, url, nil)
	if err != nil {
		return "", fhttp.StatusBadRequest, fmt.Errorf("create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	for _, c := range cookies {
		req.AddCookie(c)
	}

	response, err := client.Do(req)
	if err != nil {
		return "", fhttp.StatusBadGateway, fmt.Errorf("do request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fhttp.StatusBadGateway, fmt.Errorf("read request: %w", err)
	}

	return string(body), response.StatusCode, nil
}

func (f *Fetcher) getDomainFromURL(targetUrl string) string {
	u, err := url.Parse(targetUrl)
	if err != nil {
		return targetUrl
	}
	return u.Hostname()
}
