package config_test

import (
	"os"
	"path/filepath"
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

func TestConfig_InvalidBatchSize_ReturnsError(t *testing.T) {
	cfg := &config.Config{Transport: "stdio", Port: 3000, LogLevel: "info", EmbeddingBatchSize: 0}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding_batch_size")
}

func TestLoadFromPath_MissingFile_UsesDefaults(t *testing.T) {
	cfg, err := config.LoadFromPath(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "stdio", cfg.Transport)
	assert.Equal(t, 3000, cfg.Port)
}

func TestLoadFromPath_ValidFile_Overrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("transport: http\nport: 9090\nlog_level: debug\nembedding_batch_size: 32\n"), 0o600))

	cfg, err := config.LoadFromPath(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "http", cfg.Transport)
	assert.Equal(t, 9090, cfg.Port)
}

func TestLoadFromPath_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("transport: [\n"), 0o600))
	_, err := config.LoadFromPath(cfgPath)
	require.Error(t, err)
}

func TestLoadFromPath_InvalidValue_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("transport: grpc\nport: 3000\nlog_level: info\nembedding_batch_size: 1\n"), 0o600))
	_, err := config.LoadFromPath(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport")
}

func TestLoadFromPath_EnvOverride(t *testing.T) {
	t.Setenv("TAHU_LOG_LEVEL", "warn")
	cfg, err := config.LoadFromPath(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "warn", cfg.LogLevel)
}

func TestLoadFromPath_EnvPort(t *testing.T) {
	t.Setenv("TAHU_PORT", "8888")
	cfg, err := config.LoadFromPath(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, 8888, cfg.Port)
}

func TestLoadFromPath_EnvTransport(t *testing.T) {
	t.Setenv("TAHU_TRANSPORT", "http")
	cfg, err := config.LoadFromPath(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "http", cfg.Transport)
}

func TestLoadFromPath_EnvModel(t *testing.T) {
	t.Setenv("TAHU_EMBED_MODEL", "bm25")
	cfg, err := config.LoadFromPath(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "bm25", cfg.EmbeddingModel)
}

func TestLoadFromPath_EnvRegistry(t *testing.T) {
	t.Setenv("TAHU_REGISTRY", "/tmp/test-registry.yaml")
	cfg, err := config.LoadFromPath(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "/tmp/test-registry.yaml", cfg.BundleRegistry)
}

func TestLoad_DefaultPath(t *testing.T) {
	// Load() uses the default path (~/.tahu/config.yaml); it should succeed even
	// if the file does not exist (cold start).
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}
