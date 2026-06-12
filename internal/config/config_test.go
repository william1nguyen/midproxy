package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/william1nguyen/midproxy/internal/config"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadValidConfig(t *testing.T) {
	yaml := `
port: 3000
proxies:
  - http://proxy1.example.com
  - http://proxy2.example.com
fetch:
  timeout: 10s
solver:
  enabled: true
  timeout: 60s
redis:
  address: redis.example.com:6379
  password: secret
  db: 2
cache:
  enabled: true
  ttl: 10m
rate_limit:
  max_requests: 20
  window: 1m
`
	path := writeTemp(t, yaml)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, 3000, cfg.Port)
	require.Equal(t, []string{"http://proxy1.example.com", "http://proxy2.example.com"}, cfg.Proxies)
	require.Equal(t, 10*time.Second, cfg.Fetch.Timeout)
	require.True(t, cfg.Solver.Enabled)
	require.Equal(t, 60*time.Second, cfg.Solver.Timeout)
	require.Equal(t, "redis.example.com:6379", cfg.Redis.Address)
	require.Equal(t, "secret", cfg.Redis.Password)
	require.Equal(t, 2, cfg.Redis.DB)
	require.True(t, cfg.Cache.Enabled)
	require.Equal(t, 10*time.Minute, cfg.Cache.TTL)
	require.Equal(t, int64(20), cfg.RateLimit.MaxRequests)
	require.Equal(t, time.Minute, cfg.RateLimit.Window)
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.yaml")
	require.Error(t, err)
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeTemp(t, "port: [broken: yaml: {{{{")
	_, err := config.Load(path)
	require.Error(t, err)
}

func TestDefaultValues(t *testing.T) {
	path := writeTemp(t, "{}")
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.Port)
	require.Equal(t, 30*time.Second, cfg.Fetch.Timeout)
	require.Equal(t, 90*time.Second, cfg.Solver.Timeout)
	require.Equal(t, "localhost:6379", cfg.Redis.Address)
	require.Equal(t, 5*time.Minute, cfg.Cache.TTL)
	require.Equal(t, int64(30), cfg.RateLimit.MaxRequests)
	require.Equal(t, time.Minute, cfg.RateLimit.Window)
}

func TestOverrideDefaults(t *testing.T) {
	path := writeTemp(t, "port: 9090")
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, 9090, cfg.Port)
	require.Equal(t, 30*time.Second, cfg.Fetch.Timeout)
	require.Equal(t, 90*time.Second, cfg.Solver.Timeout)
	require.Equal(t, "localhost:6379", cfg.Redis.Address)
	require.Equal(t, 5*time.Minute, cfg.Cache.TTL)
	require.Equal(t, int64(30), cfg.RateLimit.MaxRequests)
	require.Equal(t, time.Minute, cfg.RateLimit.Window)
}

func TestDurationParsing(t *testing.T) {
	path := writeTemp(t, "fetch:\n  timeout: 2m30s")
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, 150*time.Second, cfg.Fetch.Timeout)
}
