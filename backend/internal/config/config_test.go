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

	assert.Equal(t, "9090", cfg.Server.Port)
	assert.Equal(t, "testhost", cfg.Database.Host)
	assert.Equal(t, "3307", cfg.Database.Port)
	assert.Equal(t, "testuser", cfg.Database.User)
	assert.Equal(t, "testpass", cfg.Database.Password)
	assert.Equal(t, "testdb", cfg.Database.Name)
	assert.Equal(t, "redishost", cfg.Redis.Host)
	assert.Equal(t, "6380", cfg.Redis.Port)
	assert.Equal(t, "redispass", cfg.Redis.Password)
	assert.Equal(t, "test-jwt-secret-minimum-32-chars!", cfg.JWT.Secret)
	assert.Equal(t, "example.com", cfg.WebAuthn.RPID)
	assert.Equal(t, "https://example.com", cfg.WebAuthn.RPOrigin)
	assert.Equal(t, "https://short.example.com", cfg.URLs.BaseURL)
	assert.Equal(t, "/path/to/geoip.mmdb", cfg.GeoIP.Path)
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
	assert.Contains(t, err.Error(), "jwt.secret")
}

func TestLoadYAML_FileNotFound(t *testing.T) {
	_, err := LoadFromYAML("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestLoad_ConfigNotFound(t *testing.T) {
	// Change to a directory without config.yaml
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(t.TempDir())

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config.yaml not found")
}

func TestLoadYAML_Defaults(t *testing.T) {
	// Minimal config - only required field
	content := `
jwt:
  secret: "minimum-required-secret-for-test!"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadFromYAML(configPath)
	require.NoError(t, err)

	// Check defaults are applied
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "3306", cfg.Database.Port)
	assert.Equal(t, "urlshortener", cfg.Database.User)
	assert.Equal(t, "urlshortener", cfg.Database.Name)
	assert.Equal(t, "migrations", cfg.Database.MigrationsPath)
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6379", cfg.Redis.Port)
	assert.Equal(t, "http://localhost:8080", cfg.URLs.BaseURL)
	assert.Equal(t, "localhost", cfg.WebAuthn.RPID)
	assert.Equal(t, "http://localhost:3000", cfg.WebAuthn.RPOrigin)
}

func TestApplyDefaults_URLs(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := &Config{}
		applyDefaults(cfg)

		assert.Equal(t, "http://localhost:8080", cfg.URLs.BaseURL)
		assert.Equal(t, "http://localhost:3000", cfg.URLs.FrontendURL)
		assert.Equal(t, "localhost", cfg.URLs.RedirectHost)
		assert.Equal(t, "", cfg.URLs.RedirectIP)
		require.NotNil(t, cfg.UI.FaviconService)
		assert.True(t, *cfg.UI.FaviconService)
		assert.Equal(t, "data/uploads", cfg.Uploads.Path)
	})

	t.Run("redirect host derived from base url", func(t *testing.T) {
		cfg := &Config{URLs: URLsConfig{BaseURL: "https://sho.rt:8443/x"}}
		applyDefaults(cfg)
		assert.Equal(t, "sho.rt", cfg.URLs.RedirectHost)
	})

	t.Run("explicit values kept", func(t *testing.T) {
		content := `
jwt:
  secret: "minimum-required-secret-for-test!"
urls:
  base_url: "https://s.example.com"
  frontend_url: "https://app.example.com"
  redirect_host: "cname.example.com"
  redirect_ip: "203.0.113.7"
ui:
  favicon_service: false
uploads:
  path: "/var/lib/us/uploads"
`
		configPath := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

		cfg, err := LoadFromYAML(configPath)
		require.NoError(t, err)
		assert.Equal(t, "https://app.example.com", cfg.URLs.FrontendURL)
		assert.Equal(t, "cname.example.com", cfg.URLs.RedirectHost)
		assert.Equal(t, "203.0.113.7", cfg.URLs.RedirectIP)
		require.NotNil(t, cfg.UI.FaviconService)
		assert.False(t, *cfg.UI.FaviconService)
		assert.Equal(t, "/var/lib/us/uploads", cfg.Uploads.Path)
	})
}
