package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/william1nguyen/midproxy/internal/ratelimit"
	"github.com/william1nguyen/midproxy/internal/testutil"
)

func TestTokenBucketUnderLimit(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	tb := ratelimit.NewTokenBucket(rdb, 5, 5)
	ctx := context.Background()

	for i := range 3 {
		if !tb.Allow(ctx, "under.tb") {
			t.Errorf("call %d: expected allowed", i+1)
		}
	}
}

func TestTokenBucketHitLimit(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	tb := ratelimit.NewTokenBucket(rdb, 5, 5)
	ctx := context.Background()

	for range 5 {
		tb.Allow(ctx, "hit.tb")
	}
	if tb.Allow(ctx, "hit.tb") {
		t.Error("call 6: expected rejected")
	}
}

func TestTokenBucketRefills(t *testing.T) {
	rdb := testutil.SetupRedis(t)
	tb := ratelimit.NewTokenBucket(rdb, 5, 5)
	ctx := context.Background()

	for range 5 {
		tb.Allow(ctx, "refill.tb")
	}
	if tb.Allow(ctx, "refill.tb") {
		t.Fatal("expected rejected after drain")
	}

	time.Sleep(1100 * time.Millisecond)

	if !tb.Allow(ctx, "refill.tb") {
		t.Error("expected allowed after refill")
	}
}
