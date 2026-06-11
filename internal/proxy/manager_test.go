package proxy_test

import (
	"sync"
	"testing"
	"time"

	"github.com/william1nguyen/midproxy/internal/proxy"
)

func newTestManager(urls []string) *proxy.Manager {
	return proxy.NewManager(urls, 3, 30*time.Second)
}

func TestPickRotate(t *testing.T) {
	m := newTestManager([]string{"http://a:1", "http://b:1", "http://c:1"})

	used := map[string]bool{}
	for range 5 {
		u := m.Pick()
		if u != "" {
			used[u] = true
		}
	}
	if len(used) != 3 {
		t.Errorf("expected 3 unique proxies, got %d", len(used))
	}
}

func TestCircuitOpens(t *testing.T) {
	m := newTestManager([]string{"http://a:1"})
	u := m.Pick()
	for range 3 {
		m.RecordFailure(u)
	}
	if m.Pick() != "" {
		t.Error("proxy should be unavailable after circuit opens")
	}
}

func TestRecordSuccessResetsFails(t *testing.T) {
	m := newTestManager([]string{"http://a:1"})
	u := m.Pick()
	m.RecordFailure(u)
	m.RecordFailure(u)
	m.RecordSuccess(u)
	m.RecordFailure(u)
	if m.Pick() == "" {
		t.Error("proxy should still be available after success reset")
	}
}

func TestPickEmpty(t *testing.T) {
	m := proxy.NewManager(nil, 3, 30*time.Second)
	if m.Pick() != "" {
		t.Error("expected empty string for no proxies")
	}
}

func TestAllCircuitsOpen(t *testing.T) {
	m := newTestManager([]string{"http://a:1", "http://b:1"})
	for _, u := range []string{"http://a:1", "http://b:1"} {
		for range 3 {
			m.RecordFailure(u)
		}
	}
	if m.Pick() != "" {
		t.Error("expected empty when all circuits open")
	}
}

func TestCircuitRecovery(t *testing.T) {
	m := newTestManager([]string{"http://a:1"})
	u := m.Pick()
	for range 3 {
		m.RecordFailure(u)
	}
	if m.Pick() != "" {
		t.Error("circuit should be open")
	}
	m.RecordSuccess(u)
	if m.Pick() == "" {
		t.Error("circuit should be closed after success")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := newTestManager([]string{"http://a:1", "http://b:1", "http://c:1"})
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := m.Pick()
			if u != "" {
				m.RecordSuccess(u)
				m.RecordFailure(u)
			}
		}()
	}
	wg.Wait()
}
