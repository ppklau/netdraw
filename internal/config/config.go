// Package config resolves the active project configuration.
// Resolution order: CLI flags > environment variables > .netdraw.yml config file > defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the resolved project configuration.
type Config struct {
	Adapter string `yaml:"adapter"`
	SoT     string `yaml:"sot"`
	Views   string `yaml:"views"`
	Output  string `yaml:"output"`
}

func defaults() *Config {
	return &Config{
		Adapter: "flat",
		SoT:     ".",
		Views:   "views.yml",
		Output:  "diagrams/",
	}
}

// Load resolves config from a .netdraw.yml file found in the current directory
// or any ancestor, then applies environment variable overrides.
// Call ApplyFlags afterwards to apply CLI flag values.
func Load() (*Config, error) {
	cfg := defaults()

	path, ok := findConfigFile(".")
	if !ok {
		cfg.applyEnv()
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	// Resolve relative paths against the config file's directory.
	dir := filepath.Dir(path)
	if !filepath.IsAbs(cfg.SoT) {
		cfg.SoT = filepath.Join(dir, cfg.SoT)
	}
	if !filepath.IsAbs(cfg.Views) {
		cfg.Views = filepath.Join(dir, cfg.Views)
	}
	if !filepath.IsAbs(cfg.Output) {
		cfg.Output = filepath.Join(dir, cfg.Output)
	}

	cfg.applyEnv()
	return cfg, nil
}

// ApplyFlags applies non-empty CLI flag values, overriding any earlier source.
func (c *Config) ApplyFlags(adapter, sot string) {
	if adapter != "" {
		c.Adapter = adapter
	}
	if sot != "" {
		c.SoT = sot
	}
}

func (c *Config) applyEnv() {
	if v := os.Getenv("NETDRAW_ADAPTER"); v != "" {
		c.Adapter = v
	}
	if v := os.Getenv("NETDRAW_SOT"); v != "" {
		c.SoT = v
	}
}

// findConfigFile walks from dir up to the filesystem root looking for .netdraw.yml.
func findConfigFile(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(abs, ".netdraw.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}
