package testutil

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	tcRedis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func SkipShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipped: requires docker")
	}
}

func SetupRedis(t *testing.T) *redis.Client {
	t.Helper()
	SkipShort(t)
	ctx := context.Background()

	container, err := tcRedis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	endpoint, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get redis endpoint: %v", err)
	}

	opts, err := redis.ParseURL(endpoint)
	if err != nil {
		t.Fatalf("failed to parse redis url: %v", err)
	}

	rdb := redis.NewClient(opts)
	t.Cleanup(func() { rdb.Close() })

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to ping redis: %v", err)
	}

	rdb.FlushDB(ctx)
	return rdb
}
