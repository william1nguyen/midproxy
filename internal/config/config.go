package config

import (
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Port      int             `yaml:"port"`
	Proxies   []string        `yaml:"proxies"`
	Fetch     FetchConfig     `yaml:"fetch"`
	Solver    SolverConfig    `yaml:"solver"`
	Redis     RedisConfig     `yaml:"redis"`
	Cache     CacheConfig     `yaml:"cache"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Circuit   CircuitConfig   `yaml:"circuit"`
}

type FetchConfig struct {
	Timeout        time.Duration `yaml:"timeout"`
	MaxRetries     int           `yaml:"max_retries"`
	RetryBaseDelay time.Duration `yaml:"retry_base_delay"`
	RetryMaxDelay  time.Duration `yaml:"retry_max_delay"`
}

type CircuitConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	ResetTimeout     time.Duration `yaml:"reset_timeout"`
}

type SolverConfig struct {
	Enabled bool          `yaml:"enabled"`
	Timeout time.Duration `yaml:"timeout"`
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
	MaxRPS int64 `yaml:"max_rps"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Port: 8080,
		Fetch: FetchConfig{
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			RetryBaseDelay: 1 * time.Second,
			RetryMaxDelay:  8 * time.Second,
		},
		Circuit: CircuitConfig{
			FailureThreshold: 5,
			ResetTimeout:     30 * time.Second,
		},
		Solver:    SolverConfig{Timeout: 90 * time.Second},
		Redis:     RedisConfig{Address: "localhost:6379"},
		Cache:     CacheConfig{TTL: 5 * time.Minute},
		RateLimit: RateLimitConfig{MaxRPS: 5},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}
