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
	URLs []string `yaml:"urls"`
}

func Load(configYamlFile string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{Port: 8000},
		Solver: SolverConfig{Timeout: 90 * time.Second},
		Proxy:  ProxyConfig{},
	}

	data, err := os.ReadFile(configYamlFile)
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}
