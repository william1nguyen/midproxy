package solver_test

import (
	"context"
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

	msgs, err := rdb.XRange(ctx, "stream:solve", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Values["url"] != "http://example.com" {
		t.Errorf("url = %v, want http://example.com", msgs[0].Values["url"])
	}
	if msgs[0].Values["id"] == "" {
		t.Error("expected non-empty id")
	}
}

func TestJobIDUniqueness(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	const n = 100
	for i := range n {
		domain := fmt.Sprintf("domain-%d.example.com", i)
		s.Trigger(ctx, "http://"+domain, domain, false)
	}

	msgs, err := rdb.XRange(ctx, "stream:solve", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}

	seen := make(map[string]struct{}, n)
	for _, msg := range msgs {
		id, _ := msg.Values["id"].(string)
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate job ID: %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != n {
		t.Errorf("expected %d unique IDs, got %d", n, len(seen))
	}
}

func TestJobStreamFields(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://format.example.com", "format.example.com", false)

	msgs, err := rdb.XRange(ctx, "stream:solve", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	id, ok := msgs[0].Values["id"].(string)
	if !ok || id == "" {
		t.Errorf("expected non-empty string 'id', got %v", msgs[0].Values["id"])
	}
	url, ok := msgs[0].Values["url"].(string)
	if !ok || url == "" {
		t.Errorf("expected non-empty string 'url', got %v", msgs[0].Values["url"])
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
	if ttl <= 0 || ttl > lockTTL {
		t.Errorf("expected TTL in (0, %v], got %v", lockTTL, ttl)
	}
}

func TestDuplicateTriggerSkipped(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://dup.example.com", "dup.example.com", false)
	s.Trigger(ctx, "http://dup.example.com", "dup.example.com", false)

	length := rdb.XLen(ctx, "stream:solve").Val()
	if length != 1 {
		t.Errorf("expected stream length 1, got %d", length)
	}
}

func TestForceTriggerOverwrites(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	ctx := context.Background()
	s := solver.New(rdb, lockTTL)

	s.Trigger(ctx, "http://force.example.com", "force.example.com", false)
	first := rdb.Get(ctx, "solving:force.example.com").Val()

	s.Trigger(ctx, "http://force.example.com", "force.example.com", true)
	second := rdb.Get(ctx, "solving:force.example.com").Val()

	if first == second {
		t.Error("expected lock value to change after force trigger")
	}

	length := rdb.XLen(ctx, "stream:solve").Val()
	if length != 2 {
		t.Errorf("expected stream length 2, got %d", length)
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
