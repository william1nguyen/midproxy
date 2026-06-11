package proxy_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/william1nguyen/midproxy/internal/fetch"
	"github.com/william1nguyen/midproxy/internal/proxy"
	"github.com/william1nguyen/midproxy/internal/solver"
	"github.com/william1nguyen/midproxy/internal/store"
	"github.com/william1nguyen/midproxy/internal/testutil"
)

type testEnv struct {
	upstream *httptest.Server
	proxyURL string
	client   *http.Client
	rdb      *redis.Client
}

func setup(t *testing.T, handler http.HandlerFunc) *testEnv {
	t.Helper()

	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)

	rdb := testutil.SetupRedis(t)
	st := store.New(rdb, 30*time.Second, 5)
	slv := solver.New(rdb, 30*time.Second)

	srv := proxy.NewServer(proxy.ServerConfig{
		Manager:      proxy.NewManager(nil),
		FetchClient:  fetch.NewClient(10 * time.Second),
		Store:        st,
		Solver:       slv,
		CacheEnabled: true,
	})

	proxySrv := httptest.NewServer(srv)
	t.Cleanup(proxySrv.Close)

	proxyURL := proxySrv.URL
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: func(_ *http.Request) (*url.URL, error) {
				return url.Parse(proxyURL)
			},
		},
		Timeout: 15 * time.Second,
	}

	return &testEnv{upstream: upstream, proxyURL: proxyURL, client: client, rdb: rdb}
}

func TestHealthcheck(t *testing.T) {
	env := setup(t, http.NotFoundHandler().ServeHTTP)

	resp, err := http.Get(env.proxyURL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "ok" {
		t.Errorf("got %d %q, want 200 ok", resp.StatusCode, body)
	}
}

func TestHTTPGetProxied(t *testing.T) {
	env := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "proxied")
		w.Write([]byte("hello"))
	})

	resp, err := env.client.Get(env.upstream.URL + "/test")
	if err != nil {
		t.Skipf("tls-client: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q, want hello", body)
	}
	if resp.Header.Get("X-Test") != "proxied" {
		t.Errorf("X-Test = %q, want proxied", resp.Header.Get("X-Test"))
	}
}

func TestCacheHit(t *testing.T) {
	var hits int64
	env := setup(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write([]byte("cached"))
	})

	target := env.upstream.URL + "/cacheable"

	resp1, err := env.client.Get(target)
	if err != nil {
		t.Skipf("tls-client: %v", err)
	}
	io.ReadAll(resp1.Body)
	resp1.Body.Close()

	resp2, err := env.client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.Header.Get("X-Cache") != "HIT" {
		t.Errorf("X-Cache = %q, want HIT", resp2.Header.Get("X-Cache"))
	}
	if n := atomic.LoadInt64(&hits); n != 1 {
		t.Errorf("upstream hits = %d, want 1", n)
	}
}

func TestNonGetNotCached(t *testing.T) {
	var hits int64
	env := setup(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write([]byte("ok"))
	})

	for range 2 {
		resp, err := env.client.Post(env.upstream.URL+"/post", "text/plain", strings.NewReader("body"))
		if err != nil {
			t.Skipf("tls-client: %v", err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	if n := atomic.LoadInt64(&hits); n != 2 {
		t.Errorf("upstream hits = %d, want 2", n)
	}
}

func TestRateLimited(t *testing.T) {
	env := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	got429 := false
	for range 10 {
		resp, err := env.client.Get(env.upstream.URL + "/rl")
		if err != nil {
			t.Skipf("tls-client: %v", err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Error("expected 429")
	}
}

func TestCFChallengeTriggersSolver(t *testing.T) {
	env := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `<div id="cf-browser-verification"></div>`)
	})

	resp, err := env.client.Get(env.upstream.URL + "/cf")
	if err != nil {
		t.Skipf("tls-client: %v", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != 503 {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}

	u, _ := url.Parse(env.upstream.URL)
	val, err := env.rdb.Get(context.Background(), "solving:"+u.Hostname()).Result()
	if err != nil || val == "" {
		t.Errorf("solving lock not set: err=%v val=%q", err, val)
	}
}

func TestSolveCookiesApplied(t *testing.T) {
	var gotCookie string
	env := setup(t, func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Write([]byte("ok"))
	})

	u, _ := url.Parse(env.upstream.URL)
	host := u.Hostname()

	data, _ := json.Marshal(store.SolveResult{
		UserAgent: "TestAgent",
		Cookies:   []store.Cookie{{Name: "session", Value: "abc", Domain: host, Path: "/"}},
	})
	env.rdb.LPush(context.Background(), "cookies:"+host, string(data))

	resp, err := env.client.Get(env.upstream.URL + "/solved")
	if err != nil {
		t.Skipf("tls-client: %v", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if !strings.Contains(gotCookie, "session=abc") {
		t.Errorf("cookie = %q, want session=abc", gotCookie)
	}
}
