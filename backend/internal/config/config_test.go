package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadYAML_ValidConfig(t *testing.T) {
	// Create temp YAML file
	content := `
server:
  port: "9090"
database:
  host: "testhost"
  port: "3307"
  user: "testuser"
  password: "testpass"
  name: "testdb"
redis:
  host: "redishost"
  port: "6380"
  password: "redispass"
jwt:
  secret: "test-jwt-secret-minimum-32-chars!"
webauthn:
  rp_id: "example.com"
  rp_origin: "https://example.com"
urls:
  base_url: "https://short.example.com"
geoip:
  path: "/path/to/geoip.mmdb"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromYAML(configPath)
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.ServerPort)
	assert.Equal(t, "testhost", cfg.DBHost)
	assert.Equal(t, "3307", cfg.DBPort)
	assert.Equal(t, "testuser", cfg.DBUser)
	assert.Equal(t, "testpass", cfg.DBPassword)
	assert.Equal(t, "testdb", cfg.DBName)
	assert.Equal(t, "redishost", cfg.RedisHost)
	assert.Equal(t, "6380", cfg.RedisPort)
	assert.Equal(t, "redispass", cfg.RedisPassword)
	assert.Equal(t, "test-jwt-secret-minimum-32-chars!", cfg.JWTSecret)
	assert.Equal(t, "example.com", cfg.RPID)
	assert.Equal(t, "https://example.com", cfg.RPOrigin)
	assert.Equal(t, "https://short.example.com", cfg.BaseURL)
	assert.Equal(t, "/path/to/geoip.mmdb", cfg.GeoIPPath)
}

func TestLoadYAML_MissingJWTSecret(t *testing.T) {
	content := `
server:
  port: "8080"
jwt:
  secret: ""
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	_, err = LoadFromYAML(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoadYAML_FileNotFound(t *testing.T) {
	_, err := LoadFromYAML("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestLoadYAML_EnvOverride(t *testing.T) {
	content := `
server:
  port: "8080"
jwt:
  secret: "yaml-secret-that-is-long-enough!"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	// Set env var override
	t.Setenv("SERVER_PORT", "9999")
	t.Setenv("JWT_SECRET", "env-override-secret-long-enough!")

	cfg, err := LoadFromYAML(configPath)
	require.NoError(t, err)

	// Env vars should override YAML values
	assert.Equal(t, "9999", cfg.ServerPort)
	assert.Equal(t, "env-override-secret-long-enough!", cfg.JWTSecret)
}

func TestLoad_FallbackToEnvOnly(t *testing.T) {
	// When no YAML file exists, should fall back to env vars
	t.Setenv("JWT_SECRET", "env-only-secret-long-enough!")
	t.Setenv("SERVER_PORT", "7777")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "7777", cfg.ServerPort)
	assert.Equal(t, "env-only-secret-long-enough!", cfg.JWTSecret)
}
