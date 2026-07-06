package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/infra/config"
)

func TestConfig_Validate_ValidDefaults(t *testing.T) {
	cfg := &config.Config{
		Transport:          "stdio",
		Port:               3000,
		LogLevel:           "info",
		EmbeddingBatchSize: 64,
	}
	require.NoError(t, cfg.Validate())
}

func TestConfig_InvalidTransport_ReturnsError(t *testing.T) {
	cfg := &config.Config{Transport: "grpc", Port: 3000, LogLevel: "info", EmbeddingBatchSize: 1}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport")
}

func TestConfig_InvalidPort_ReturnsError(t *testing.T) {
	cfg := &config.Config{Transport: "stdio", Port: 0, LogLevel: "info", EmbeddingBatchSize: 1}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestConfig_InvalidLogLevel_ReturnsError(t *testing.T) {
	cfg := &config.Config{Transport: "stdio", Port: 3000, LogLevel: "verbose", EmbeddingBatchSize: 1}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_level")
}

func TestConfig_HTTP_ValidTransport(t *testing.T) {
	cfg := &config.Config{Transport: "http", Port: 8080, LogLevel: "debug", EmbeddingBatchSize: 32}
	require.NoError(t, cfg.Validate())
}
