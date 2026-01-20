package goqualgate

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultConfigFile is the default path to the goqualgate config file.
const DefaultConfigFile = ".goqualgate.yaml"

// Config holds all goqualgate configuration.
type Config struct {
	Coverage CoverageYAML `yaml:"coverage"`
}

// CoverageYAML holds coverage settings from the config file.
type CoverageYAML struct {
	Threshold           float64 `yaml:"threshold"`
	NewPackageThreshold float64 `yaml:"new_package_threshold"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config YAML: %w", err)
	}
	return &cfg, nil
}
