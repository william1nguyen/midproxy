package proxy_test

import (
	"testing"

	"github.com/william1nguyen/midproxy/internal/proxy"
)

func TestPickRotate(t *testing.T) {
	m := proxy.NewManager([]string{
		"http://a:1",
		"http://b:1",
		"http://c:1",
	})

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

func TestPickCooldownSkipped(t *testing.T) {
	m := proxy.NewManager([]string{
		"http://a:1",
	})
	u := m.Pick()
	for i := 0; i < 3; i++ {
		m.RecordFailure(u)
	}
	if m.Pick() != "" {
		t.Errorf("proxy in cooldown should not be returned")
	}
}

func TestRecordSuccessResetsFails(t *testing.T) {
	m := proxy.NewManager([]string{
		"http://a:1",
	})
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
	m := proxy.NewManager(nil)
	if m.Pick() != "" {
		t.Error("expected empty string for no proxies")
	}
}
