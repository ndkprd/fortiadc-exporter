// Package config loads and validates the exporter's YAML configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	defaultListenAddress = ":9634"
	defaultVDOM          = "root"
)

// Config is the top-level exporter configuration. HTTP paths (/probe,
// /metrics) are fixed exporter conventions, not configurable here.
type Config struct {
	FortiADC      map[string]Target `yaml:"fortiadc"`
	ListenAddress string            `yaml:"listen_address"`
}

// Target holds one FortiADC appliance's connection details. APIKey is the
// key of a FortiADC REST API administrator; APIKeyEnv, when set, names an
// environment variable that overrides it.
type Target struct {
	URL                string `yaml:"url"`
	APIKey             string `yaml:"api_key"`
	APIKeyEnv          string `yaml:"api_key_env"`
	VDOM               string `yaml:"vdom"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

// Load reads and parses the YAML config file at path, applying defaults and
// validating the required target fields.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if cfg.ListenAddress == "" {
		cfg.ListenAddress = defaultListenAddress
	}
	if len(cfg.FortiADC) == 0 {
		return nil, fmt.Errorf("validating config file %q: at least one fortiadc target must be configured", path)
	}
	for name, t := range cfg.FortiADC {
		if t.APIKeyEnv != "" {
			if v := os.Getenv(t.APIKeyEnv); v != "" {
				t.APIKey = v
			}
		}
		if t.VDOM == "" {
			t.VDOM = defaultVDOM
		}
		if t.URL == "" {
			return nil, fmt.Errorf("validating config file %q: target %q: url is required", path, name)
		}
		if t.APIKey == "" {
			return nil, fmt.Errorf("validating config file %q: target %q: api_key is required", path, name)
		}
		cfg.FortiADC[name] = t
	}

	return &cfg, nil
}
