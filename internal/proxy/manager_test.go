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
		p := m.Pick()
		if p != nil {
			used[p.URL] = true
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
	p := m.Pick()
	for i := 0; i < 3; i++ {
		m.RecordFailure(p.URL)
	}
	if m.Pick() != nil {
		t.Errorf("proxy in cooldown should not be returned")
	}
}

func TestRecordSuccessResetsFails(t *testing.T) {
	m := proxy.NewManager([]string{
		"http://a:1",
	})
	p := m.Pick()
	m.RecordFailure(p.URL)
	m.RecordFailure(p.URL)
	m.RecordSuccess(p.URL)
	m.RecordFailure(p.URL)
	if m.Pick() == nil {
		t.Error("proxy should still be available after success reset")
	}
}
