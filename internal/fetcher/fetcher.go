package fetcher

import (
	"context"
	"fmt"
	"io"

	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Fetcher struct {
	timeout time.Duration
}

func New(timeout time.Duration) *Fetcher {
	return &Fetcher{timeout: timeout}
}

func (fetcher *Fetcher) Fetch(ctx context.Context, url string, headers map[string]string, proxyURL string, cookies []*fhttp.Cookie) (string, int, error) {
	opts := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(int(fetcher.timeout.Seconds())),
		tls_client.WithClientProfile(profiles.Chrome_120),
	}

	if proxyURL != "" {
		opts = append(opts, tls_client.WithProxyUrl(proxyURL))
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
