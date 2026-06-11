package store_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/william1nguyen/midproxy/internal/store"
	"github.com/william1nguyen/midproxy/internal/testutil"
)

// --- Cache tests ---

func TestCacheSetAndGet(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	header := http.Header{"Content-Type": []string{"application/json"}}
	body := []byte(`{"hello":"world"}`)
	resp := store.EncodeCachedResponse(200, header, body)

	if err := s.SetCachedResponse(ctx, "GET", "http://example.com/test", resp); err != nil {
		t.Fatalf("SetCachedResponse: %v", err)
	}

	got, err := s.GetCachedResponse(ctx, "GET", "http://example.com/test")
	if err != nil {
		t.Fatalf("GetCachedResponse: %v", err)
	}
	if got.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", got.StatusCode)
	}
	if got.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", got.Header.Get("Content-Type"))
	}
	decoded, err := got.DecodeBody()
	if err != nil {
		t.Fatalf("DecodeBody: %v", err)
	}
	if string(decoded) != string(body) {
		t.Errorf("expected body %q, got %q", body, decoded)
	}
}

func TestCacheMiss(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	_, err := s.GetCachedResponse(ctx, "GET", "http://never-set.example.com")
	if err == nil {
		t.Fatal("expected error for cache miss, got nil")
	}
}

func TestCacheTTLExpired(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 1*time.Second, 100)
	ctx := context.Background()

	resp := store.EncodeCachedResponse(200, http.Header{}, []byte("body"))
	if err := s.SetCachedResponse(ctx, "GET", "http://ttl.example.com", resp); err != nil {
		t.Fatalf("SetCachedResponse: %v", err)
	}

	time.Sleep(2 * time.Second)

	_, err := s.GetCachedResponse(ctx, "GET", "http://ttl.example.com")
	if err == nil {
		t.Fatal("expected error after TTL expiry, got nil")
	}
}

func TestCacheOverwrite(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	first := store.EncodeCachedResponse(200, http.Header{}, []byte("first"))
	second := store.EncodeCachedResponse(201, http.Header{}, []byte("second"))

	if err := s.SetCachedResponse(ctx, "GET", "http://overwrite.example.com", first); err != nil {
		t.Fatalf("SetCachedResponse first: %v", err)
	}
	if err := s.SetCachedResponse(ctx, "GET", "http://overwrite.example.com", second); err != nil {
		t.Fatalf("SetCachedResponse second: %v", err)
	}

	got, err := s.GetCachedResponse(ctx, "GET", "http://overwrite.example.com")
	if err != nil {
		t.Fatalf("GetCachedResponse: %v", err)
	}
	if got.StatusCode != 201 {
		t.Errorf("expected status 201 after overwrite, got %d", got.StatusCode)
	}
	decoded, err := got.DecodeBody()
	if err != nil {
		t.Fatalf("DecodeBody: %v", err)
	}
	if string(decoded) != "second" {
		t.Errorf("expected body %q, got %q", "second", decoded)
	}
}

// --- SolveResult tests ---

func TestGetSolveResultNoData(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	_, err := s.GetSolveResult(ctx, "missing-domain.com")
	if err == nil {
		t.Fatal("expected error for missing domain, got nil")
	}
}

func TestStoreThenGetSolveResult(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	payload := `{"userAgent":"TestAgent","cookies":[{"name":"n","value":"v","domain":"test.com","path":"/"}],"proxyURL":"http://p:1"}`
	if err := rdb.LPush(ctx, "cookies:test.com", payload).Err(); err != nil {
		t.Fatalf("LPush: %v", err)
	}

	result, err := s.GetSolveResult(ctx, "test.com")
	if err != nil {
		t.Fatalf("GetSolveResult: %v", err)
	}
	if result.UserAgent != "TestAgent" {
		t.Errorf("expected UserAgent %q, got %q", "TestAgent", result.UserAgent)
	}
	if result.ProxyURL != "http://p:1" {
		t.Errorf("expected ProxyURL %q, got %q", "http://p:1", result.ProxyURL)
	}
	if len(result.Cookies) != 1 || result.Cookies[0].Name != "n" {
		t.Errorf("unexpected cookies: %+v", result.Cookies)
	}
}

func TestSolveResultRotation(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	type entry struct{ UA string }
	push := func(ua string) {
		data, _ := json.Marshal(map[string]interface{}{
			"userAgent": ua,
			"cookies":   []interface{}{},
			"proxyURL":  "http://p:1",
		})
		if err := rdb.LPush(ctx, "cookies:rot.com", string(data)).Err(); err != nil {
			t.Fatalf("LPush %s: %v", ua, err)
		}
	}

	// LPush prepends; LMOVE RIGHT→LEFT takes from the tail.
	// Push A first, then B, then C so the list is [C, B, A] (head→tail),
	// meaning A is at the right (tail). Rotation order: A, B, C, A, ...
	push("A")
	push("B")
	push("C")

	expected := []string{"A", "B", "C", "A"}
	for i, want := range expected {
		result, err := s.GetSolveResult(ctx, "rot.com")
		if err != nil {
			t.Fatalf("call %d: GetSolveResult: %v", i+1, err)
		}
		if result.UserAgent != want {
			t.Errorf("call %d: expected UserAgent %q, got %q", i+1, want, result.UserAgent)
		}
	}
}

func TestSolveResultTTL(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	payload := `{"userAgent":"TTLAgent","cookies":[],"proxyURL":"http://p:1"}`
	if err := rdb.LPush(ctx, "cookies:ttl.com", payload).Err(); err != nil {
		t.Fatalf("LPush: %v", err)
	}
	if err := rdb.Expire(ctx, "cookies:ttl.com", 1*time.Second).Err(); err != nil {
		t.Fatalf("Expire: %v", err)
	}

	time.Sleep(2 * time.Second)

	_, err := s.GetSolveResult(ctx, "ttl.com")
	if err == nil {
		t.Fatal("expected error after key TTL expiry, got nil")
	}
}

// --- Rate Limit tests ---

func TestAllowRequestUnderLimit(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 5)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		if !s.AllowRequest(ctx, "under.example.com") {
			t.Errorf("call %d: expected true (under limit), got false", i)
		}
	}
}

func TestAllowRequestHitLimit(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 5)
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		if !s.AllowRequest(ctx, "hit.example.com") {
			t.Errorf("call %d: expected true (within limit), got false", i)
		}
	}
	if s.AllowRequest(ctx, "hit.example.com") {
		t.Error("call 6: expected false (over limit), got true")
	}
}

func TestAllowRequestResetAfterWindow(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 5)
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		s.AllowRequest(ctx, "reset.example.com")
	}
	if s.AllowRequest(ctx, "reset.example.com") {
		t.Fatal("expected false after hitting limit, got true")
	}

	time.Sleep(1100 * time.Millisecond)

	if !s.AllowRequest(ctx, "reset.example.com") {
		t.Error("expected true after window reset, got false")
	}
}

// --- Invalidate test ---

func TestInvalidateSolveResult(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	s := store.New(rdb, 10*time.Second, 100)
	ctx := context.Background()

	payload := `{"userAgent":"InvAgent","cookies":[],"proxyURL":"http://p:1"}`
	if err := rdb.LPush(ctx, "cookies:inv.com", payload).Err(); err != nil {
		t.Fatalf("LPush: %v", err)
	}

	s.InvalidateSolveResult(ctx, "inv.com")

	_, err := s.GetSolveResult(ctx, "inv.com")
	if err == nil {
		t.Fatal("expected error after invalidation, got nil")
	}
}
