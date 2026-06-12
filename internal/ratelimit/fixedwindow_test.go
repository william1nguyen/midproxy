package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/william1nguyen/midproxy/internal/ratelimit"
	"github.com/william1nguyen/midproxy/internal/testutil"
)

func TestFixedWindowUnderLimit(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	fw := ratelimit.NewFixedWindow(rdb, 5, time.Second)
	ctx := context.Background()

	for i := range 3 {
		if !fw.Allow(ctx, "under.test") {
			t.Errorf("call %d: expected allowed", i+1)
		}
	}
}

func TestFixedWindowHitLimit(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	fw := ratelimit.NewFixedWindow(rdb, 5, time.Second)
	ctx := context.Background()

	for range 5 {
		fw.Allow(ctx, "hit.test")
	}
	if fw.Allow(ctx, "hit.test") {
		t.Error("call 6: expected rejected")
	}
}

func TestFixedWindowResetAfterWindow(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	fw := ratelimit.NewFixedWindow(rdb, 5, time.Second)
	ctx := context.Background()

	for range 5 {
		fw.Allow(ctx, "reset.test")
	}
	if fw.Allow(ctx, "reset.test") {
		t.Fatal("expected rejected after limit")
	}

	time.Sleep(1100 * time.Millisecond)

	if !fw.Allow(ctx, "reset.test") {
		t.Error("expected allowed after window reset")
	}
}
