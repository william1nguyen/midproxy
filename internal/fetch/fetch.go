package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsClient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/william1nguyen/midproxy/internal/store"
)

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type Client struct {
	mu      sync.RWMutex
	clients map[string]tlsClient.HttpClient
	timeout time.Duration
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		clients: make(map[string]tlsClient.HttpClient),
		timeout: timeout,
	}
}

func (c *Client) getOrCreate(proxyURL string) (tlsClient.HttpClient, error) {
	c.mu.RLock()
	client, ok := c.clients[proxyURL]
	c.mu.RUnlock()
	if ok {
		return client, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if client, ok = c.clients[proxyURL]; ok {
		return client, nil
	}

	opts := []tlsClient.HttpClientOption{
		tlsClient.WithTimeoutSeconds(int(c.timeout.Seconds())),
		tlsClient.WithClientProfile(profiles.Chrome_120),
	}
	if proxyURL != "" {
		opts = append(opts, tlsClient.WithProxyUrl(proxyURL))
	}

	client, err := tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), opts...)
	if err != nil {
		return nil, err
	}
	c.clients[proxyURL] = client
	return client, nil
}

// Forward sends an HTTP request through the TLS client with Chrome fingerprint.
func (c *Client) Forward(ctx context.Context, r *http.Request, proxyURL string, cookies []store.Cookie) (*Response, error) {
	client, err := c.getOrCreate(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("create tls client: %w", err)
	}

	req, err := fhttp.NewRequestWithContext(ctx, r.Method, r.URL.String(), r.Body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, vv := range r.Header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	// remove hop-by-hop headers
	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authorization")

	for _, cookie := range cookies {
		req.AddCookie(&fhttp.Cookie{Name: cookie.Name, Value: cookie.Value})
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	header := make(http.Header)
	for k, vv := range resp.Header {
		for _, v := range vv {
			header.Add(k, v)
		}
	}

	return &Response{StatusCode: resp.StatusCode, Header: header, Body: body}, nil
}

func IsCloudflareChallenge(statusCode int, body []byte) bool {
	if statusCode != 403 {
		return false
	}
	s := string(body)
	return strings.Contains(s, "cf-browser-verification") ||
		strings.Contains(s, "cf_chl_opt") ||
		strings.Contains(s, "challenge-platform")
}
