package config

import (
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Solver    SolverConfig    `yaml:"solver"`
	Proxy     ProxyConfig     `yaml:"proxy"`
	Redis     RedisConfig     `yaml:"redis"`
	Cache     CacheConfig     `yaml:"cache"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

type ServerConfig struct {
	Port    int           `yaml:"port"`
	Fetcher FetcherConfig `yaml:"fetcher"`
}

type FetcherConfig struct {
	Timeout time.Duration `yaml:"timeout"`
}

type SolverConfig struct {
	Nodes   []string      `yaml:"nodes"`
	Timeout time.Duration `yaml:"timeout"`
}

type ProxyConfig struct {
	URLs        []string          `yaml:"urls"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type CacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
}

type RateLimitConfig struct {
	MaxRPSPerDomain int64 `yaml:"max_rps_per_domain"`
}

func Load(configYamlFile string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8000,
			Fetcher: FetcherConfig{
				Timeout: 30 * time.Second,
			},
		},
		Solver: SolverConfig{Timeout: 90 * time.Second},
		Proxy: ProxyConfig{
			HealthCheck: HealthCheckConfig{Interval: 30 * time.Second, Timeout: 10 * time.Second},
		},
		Redis: RedisConfig{
			Address: "localhost:6379",
		},
		Cache:     CacheConfig{TTL: 5 * time.Minute},
		RateLimit: RateLimitConfig{MaxRPSPerDomain: 2},
	}

	data, err := os.ReadFile(configYamlFile)
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}
