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
)

type Fetcher struct {
	timeout time.Duration
	mutex   sync.RWMutex
	clients map[string]tlsClient.HttpClient
}

func New(timeout time.Duration) *Fetcher {
	return &Fetcher{
		timeout: timeout,
		clients: make(map[string]tlsClient.HttpClient),
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

func (f *Fetcher) Fetch(ctx context.Context, url string, headers map[string]string, proxyURL string, cookies []*fhttp.Cookie) (string, int, error) {
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
