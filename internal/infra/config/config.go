// Package config loads daemon configuration from ~/.tahu/config.yaml
// with optional environment-variable overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all runtime configuration for the tahu daemon.
type Config struct {
	// Transport selects the MCP protocol transport: "stdio" or "http".
	Transport string `yaml:"transport"`

	// Port is the TCP port used in HTTP transport mode. Default 3000.
	Port int `yaml:"port"`

	// BindAddr is the interface address for HTTP transport. Default "127.0.0.1".
	BindAddr string `yaml:"bind_addr"`

	// BundleRegistry is the path to the YAML file that persists registered bundles.
	BundleRegistry string `yaml:"bundle_registry"`

	// EmbeddingModel selects the embedding backend: "bm25" or "minilm-l6-v2".
	EmbeddingModel string `yaml:"embedding_model"`

	// EmbeddingBatchSize controls how many texts are embedded in one call.
	EmbeddingBatchSize int `yaml:"embedding_batch_size"`

	// HNSWEfConstruction is the HNSW EfSearch parameter (graph quality).
	HNSWEfConstruction int `yaml:"hnsw_ef_construction"`

	// HNSWM is the HNSW M parameter (maximum neighbours per node).
	HNSWM int `yaml:"hnsw_m"`

	// LogLevel controls slog verbosity: "debug", "info", "warn", "error".
	LogLevel string `yaml:"log_level"`
}

// Load reads configuration from ~/.tahu/config.yaml, then applies env
// overrides (TAHU_TRANSPORT, TAHU_PORT, TAHU_LOG_LEVEL, TAHU_EMBED_MODEL,
// TAHU_REGISTRY). Missing config file is treated as an empty file; all
// settings fall back to defaults.
func Load() (*Config, error) {
	cfg := defaults()

	cfgPath := filepath.Join(homeDir(), ".tahu", "config.yaml")
	data, err := os.ReadFile(cfgPath) //nolint:gosec // cfgPath is from UserHomeDir, not user input
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config.Load: read %s: %w", cfgPath, err)
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config.Load: parse %s: %w", cfgPath, err)
		}
	}

	applyEnv(cfg)
	return cfg, nil
}

// defaults returns a Config with all default values populated.
func defaults() *Config {
	home := homeDir()
	return &Config{
		Transport:          "stdio",
		Port:               3000,
		BindAddr:           "127.0.0.1",
		BundleRegistry:     filepath.Join(home, ".tahu", "registry.yaml"),
		EmbeddingModel:     "bm25",
		EmbeddingBatchSize: 64,
		HNSWEfConstruction: 200,
		HNSWM:              16,
		LogLevel:           "info",
	}
}

// applyEnv overlays environment variable values onto cfg.
func applyEnv(cfg *Config) {
	if v := os.Getenv("TAHU_TRANSPORT"); v != "" {
		cfg.Transport = v
	}
	if v := os.Getenv("TAHU_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("TAHU_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("TAHU_EMBED_MODEL"); v != "" {
		cfg.EmbeddingModel = v
	}
	if v := os.Getenv("TAHU_REGISTRY"); v != "" {
		cfg.BundleRegistry = v
	}
}

// homeDir returns the user's home directory, or "." if unavailable.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}
