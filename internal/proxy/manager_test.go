package proxy_test

import (
	"testing"

	"github.com/william1nguyen/midproxy/internal/proxy"
)

func TestPickRotate(t *testing.T) {
	manager := proxy.NewManager([]string{
		"http://a:1",
		"http://b:1",
		"http://c:1",
	})

	used := map[string]bool{}
	for i := 0; i < 5; i++ {
		proxy := manager.Pick()
		if proxy != nil {
			used[proxy.URL] = true
		}
	}
	if len(used) != 3 {
		t.Errorf("expected 3 unique proxies, got %d", len(used))
	}
}

func TestPickCooldownSkipped(t *testing.T) {
	manager := proxy.NewManager([]string{
		"http://a:1",
	})
	proxy := manager.Pick()
	for i := 0; i < 3; i++ {
		manager.RecordFailure(proxy.URL)
	}
	if manager.Pick() != nil {
		t.Errorf("proxy in cooldown should not be returned")
	}
}

func TestRecordSuccessResetsFails(t *testing.T) {
	manager := proxy.NewManager([]string{
		"http://a:1",
	})
	proxy := manager.Pick()
	manager.RecordFailure(proxy.URL)
	manager.RecordFailure(proxy.URL)
	manager.RecordSuccess(proxy.URL)
	manager.RecordFailure(proxy.URL)
	if manager.Pick() == nil {
		t.Error("proxy should still be available after success reset")
	}
}
