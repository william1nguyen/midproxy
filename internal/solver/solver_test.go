package solver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/william1nguyen/midproxy/internal/solver"
	"github.com/william1nguyen/midproxy/internal/testutil"
)

const lockTTL = 30 * time.Second

func TestTriggerPushesJob(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://example.com", "example.com", false)

	raw, err := rdb.RPop(ctx, "queue:solve").Result()
	if err != nil {
		t.Fatalf("RPop: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	id, ok := m["id"]
	if !ok || id == "" {
		t.Errorf("expected non-empty 'id' field, got %v", id)
	}

	url, ok := m["url"]
	if !ok || url == "" {
		t.Errorf("expected non-empty 'url' field, got %v", url)
	}
}

func TestJobIDUniqueness(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	const n = 100
	for i := 0; i < n; i++ {
		domain := fmt.Sprintf("domain-%d.example.com", i)
		s.Trigger(ctx, "http://"+domain, domain, false)
	}

	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		raw, err := rdb.RPop(ctx, "queue:solve").Result()
		if err != nil {
			t.Fatalf("RPop iteration %d: %v", i, err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("json.Unmarshal iteration %d: %v", i, err)
		}
		id, _ := m["id"].(string)
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate job ID found: %q", id)
		}
		seen[id] = struct{}{}
	}

	if len(seen) != n {
		t.Errorf("expected %d unique IDs, got %d", n, len(seen))
	}
}

func TestJobJSONFormat(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://format.example.com", "format.example.com", false)

	raw, err := rdb.RPop(ctx, "queue:solve").Result()
	if err != nil {
		t.Fatalf("RPop: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	id, ok := m["id"].(string)
	if !ok || id == "" {
		t.Errorf("expected 'id' to be a non-empty string, got %T %v", m["id"], m["id"])
	}

	url, ok := m["url"].(string)
	if !ok || url == "" {
		t.Errorf("expected 'url' to be a non-empty string, got %T %v", m["url"], m["url"])
	}
}

func TestTriggerSetsLock(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://lock.example.com", "lock.example.com", false)

	ttl, err := rdb.TTL(ctx, "solving:lock.example.com").Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}

	if ttl <= 0 {
		t.Errorf("expected TTL > 0, got %v", ttl)
	}
	if ttl > lockTTL {
		t.Errorf("expected TTL <= lockTTL (%v), got %v", lockTTL, ttl)
	}
}

func TestDuplicateTriggerSkipped(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://dup.example.com", "dup.example.com", false)
	s.Trigger(ctx, "http://dup.example.com", "dup.example.com", false)

	length, err := rdb.LLen(ctx, "queue:solve").Result()
	if err != nil {
		t.Fatalf("LLen: %v", err)
	}
	if length != 1 {
		t.Errorf("expected queue length 1, got %d", length)
	}
}

func TestForceTriggerOverwrites(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://force.example.com", "force.example.com", false)

	firstLockVal, err := rdb.Get(ctx, "solving:force.example.com").Result()
	if err != nil {
		t.Fatalf("Get after first trigger: %v", err)
	}

	s.Trigger(ctx, "http://force.example.com", "force.example.com", true)

	secondLockVal, err := rdb.Get(ctx, "solving:force.example.com").Result()
	if err != nil {
		t.Fatalf("Get after force trigger: %v", err)
	}

	if firstLockVal == secondLockVal {
		t.Errorf("expected lock value to change after force trigger, both are %q", firstLockVal)
	}

	length, err := rdb.LLen(ctx, "queue:solve").Result()
	if err != nil {
		t.Fatalf("LLen: %v", err)
	}
	if length != 2 {
		t.Errorf("expected queue length 2, got %d", length)
	}
}

func TestRetryAfterDecreases(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	first := s.Trigger(ctx, "http://retry.example.com", "retry.example.com", false)

	time.Sleep(3 * time.Second)

	second := s.Trigger(ctx, "http://retry.example.com", "retry.example.com", false)

	if second >= first {
		t.Errorf("expected retry-after to decrease: first=%d, second=%d", first, second)
	}
}
