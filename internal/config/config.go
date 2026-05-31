package config

import (
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Solver SolverConfig `yaml:"solver"`
	Proxy  ProxyConfig  `yaml:"proxy"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
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

func Load(configYamlFile string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{Port: 8000},
		Solver: SolverConfig{Timeout: 90 * time.Second},
		Proxy: ProxyConfig{
			HealthCheck: HealthCheckConfig{Interval: 30 * time.Second, Timeout: 10 * time.Second},
		},
	}

	data, err := os.ReadFile(configYamlFile)
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}
