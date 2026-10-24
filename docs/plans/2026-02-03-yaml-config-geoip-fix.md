# YAML Configuration & GeoIP Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Convert backend configuration from environment variables to YAML file, and fix GeoIP initialization so location data displays correctly.

**Architecture:** Replace godotenv-based config with YAML parsing using gopkg.in/yaml.v3. Add GeoIP initialization call in main.go after config is loaded. Keep environment variable override capability for deployment flexibility.

**Tech Stack:** Go, gopkg.in/yaml.v3 (already indirect dep), MaxMind GeoIP2

---

## Task 1: Create YAML Config Structure

**Files:**
- Create: `backend/config.yaml`
- Create: `backend/config.example.yaml`

**Step 1: Create example config file**

Create `backend/config.example.yaml`:

```yaml
# URL Shortener Configuration
# Copy this file to config.yaml and update values

server:
  port: "8080"

database:
  host: "localhost"
  port: "3306"
  user: "urlshortener"
  password: ""
  name: "urlshortener"

redis:
  host: "localhost"
  port: "6379"
  password: ""

jwt:
  secret: "" # REQUIRED - generate a secure random string

webauthn:
  rp_id: "localhost"
  rp_origin: "http://localhost:3000"

urls:
  base_url: "http://localhost:8080"

geoip:
  # Path to MaxMind GeoIP2 City database (.mmdb file)
  # Download from: https://www.maxmind.com/en/geoip2-databases
  # Leave empty to disable GeoIP lookups
  path: ""
```

**Step 2: Copy to config.yaml for local dev**

Run: `cp backend/config.example.yaml backend/config.yaml`

**Step 3: Add config.yaml to .gitignore**

Ensure `backend/config.yaml` is in `.gitignore` (contains secrets).

**Step 4: Commit**

```bash
git add backend/config.example.yaml
git commit -m "feat(config): add YAML config example file"
```

---

## Task 2: Update Config Package to Support YAML

**Files:**
- Modify: `backend/internal/config/config.go`
- Create: `backend/internal/config/config_test.go`

**Step 1: Write the failing test**

Create `backend/internal/config/config_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test -v ./internal/config/...`
Expected: FAIL - `LoadFromYAML` not defined

**Step 3: Write minimal implementation**

Update `backend/internal/config/config.go`:

```go
// backend/internal/config/config.go
package config

import (
	"errors"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerPort    string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	RedisHost     string
	RedisPort     string
	RedisPassword string
	JWTSecret     string
	BaseURL       string
	RPID          string
	RPOrigin      string
	GeoIPPath     string
}

// yamlConfig mirrors the YAML structure
type yamlConfig struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Name     string `yaml:"name"`
	} `yaml:"database"`
	Redis struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Password string `yaml:"password"`
	} `yaml:"redis"`
	JWT struct {
		Secret string `yaml:"secret"`
	} `yaml:"jwt"`
	WebAuthn struct {
		RPID     string `yaml:"rp_id"`
		RPOrigin string `yaml:"rp_origin"`
	} `yaml:"webauthn"`
	URLs struct {
		BaseURL string `yaml:"base_url"`
	} `yaml:"urls"`
	GeoIP struct {
		Path string `yaml:"path"`
	} `yaml:"geoip"`
}

// LoadFromYAML loads configuration from a YAML file with env var overrides
func LoadFromYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return nil, err
	}

	cfg := &Config{
		ServerPort:    getEnvOrDefault("SERVER_PORT", yc.Server.Port, "8080"),
		DBHost:        getEnvOrDefault("DB_HOST", yc.Database.Host, "localhost"),
		DBPort:        getEnvOrDefault("DB_PORT", yc.Database.Port, "3306"),
		DBUser:        getEnvOrDefault("DB_USER", yc.Database.User, "urlshortener"),
		DBPassword:    getEnvOrDefault("DB_PASSWORD", yc.Database.Password, ""),
		DBName:        getEnvOrDefault("DB_NAME", yc.Database.Name, "urlshortener"),
		RedisHost:     getEnvOrDefault("REDIS_HOST", yc.Redis.Host, "localhost"),
		RedisPort:     getEnvOrDefault("REDIS_PORT", yc.Redis.Port, "6379"),
		RedisPassword: getEnvOrDefault("REDIS_PASSWORD", yc.Redis.Password, ""),
		JWTSecret:     getEnvOrDefault("JWT_SECRET", yc.JWT.Secret, ""),
		BaseURL:       getEnvOrDefault("BASE_URL", yc.URLs.BaseURL, "http://localhost:8080"),
		RPID:          getEnvOrDefault("RP_ID", yc.WebAuthn.RPID, "localhost"),
		RPOrigin:      getEnvOrDefault("RP_ORIGIN", yc.WebAuthn.RPOrigin, "http://localhost:3000"),
		GeoIPPath:     getEnvOrDefault("GEOIP_PATH", yc.GeoIP.Path, ""),
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET environment variable is required")
	}

	return cfg, nil
}

// Load tries YAML config first, falls back to env-only
func Load() (*Config, error) {
	// Try YAML config first
	yamlPaths := []string{"config.yaml", "../config.yaml", "../../config.yaml"}
	for _, path := range yamlPaths {
		if _, err := os.Stat(path); err == nil {
			return LoadFromYAML(path)
		}
	}

	// Fall back to env-only config
	return loadFromEnv()
}

// loadFromEnv loads config from environment variables only (legacy behavior)
func loadFromEnv() (*Config, error) {
	cfg := &Config{
		ServerPort:    getEnv("SERVER_PORT", "8080"),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnv("DB_PORT", "3306"),
		DBUser:        getEnv("DB_USER", "urlshortener"),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBName:        getEnv("DB_NAME", "urlshortener"),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		JWTSecret:     getEnv("JWT_SECRET", ""),
		BaseURL:       getEnv("BASE_URL", "http://localhost:8080"),
		RPID:          getEnv("RP_ID", "localhost"),
		RPOrigin:      getEnv("RP_ORIGIN", "http://localhost:3000"),
		GeoIPPath:     getEnv("GEOIP_PATH", ""),
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET environment variable is required")
	}

	return cfg, nil
}

// getEnvOrDefault returns env var if set, otherwise yaml value, otherwise default
func getEnvOrDefault(envKey, yamlValue, defaultValue string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	if yamlValue != "" {
		return yamlValue
	}
	return defaultValue
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test -v ./internal/config/...`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/config/config.go backend/internal/config/config_test.go
git commit -m "feat(config): add YAML configuration support with env var overrides"
```

---

## Task 3: Initialize GeoIP in main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

**Step 1: Write integration test concept**

The GeoIP initialization will be tested via manual verification (check logs show GeoIP status).

**Step 2: Update main.go to initialize GeoIP**

Add the GeoIP initialization after config load in `backend/cmd/server/main.go`. The key changes:

1. Add import for `util` package
2. Call `util.InitGeoIP()` after config load
3. Add `defer util.CloseGeoIP()`

Update the imports section to add:
```go
"github.com/SeaCodeBase/urlshortener/internal/util"
```

After the line `logger.Init(true)` and before database connection, add:

```go
// Initialize GeoIP (optional - logs warning if path invalid)
if err := util.InitGeoIP(ctx, cfg.GeoIPPath); err != nil {
	logger.Warn(ctx, "geoip initialization failed - location lookups disabled",
		zap.String("path", cfg.GeoIPPath),
		zap.Error(err),
	)
}
defer util.CloseGeoIP()
```

**Step 3: Run the server to verify**

Run: `cd backend && go build -o server ./cmd/server && ./server`
Expected: Server starts, logs show GeoIP status (disabled if no path, or initialized if path provided)

**Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(geoip): initialize GeoIP database on startup"
```

---

## Task 4: Add GeoIP Initialization Logging

**Files:**
- Modify: `backend/internal/util/geoip.go`

**Step 1: Add success logging to InitGeoIP**

The current `InitGeoIP` function only logs errors. Add info log on successful initialization.

After line 42 (`geoIPInstance = &GeoIPLookup{db: db}`), add:

```go
logger.Info(ctx, "geoip: database initialized successfully",
	zap.String("path", dbPath),
)
```

**Step 2: Run to verify logging**

Run: `cd backend && go build -o server ./cmd/server`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add backend/internal/util/geoip.go
git commit -m "feat(geoip): add success logging on initialization"
```

---

## Task 5: Update .gitignore

**Files:**
- Modify: `.gitignore` (root)

**Step 1: Add config.yaml to gitignore**

Add to `.gitignore`:

```
# Backend config with secrets
backend/config.yaml
```

**Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add config.yaml to gitignore"
```

---

## Task 6: Ensure gopkg.in/yaml.v3 is Direct Dependency

**Files:**
- Modify: `backend/go.mod`

**Step 1: Add yaml dependency explicitly**

Run: `cd backend && go get gopkg.in/yaml.v3`

**Step 2: Tidy dependencies**

Run: `cd backend && go mod tidy`

**Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore(deps): add gopkg.in/yaml.v3 as direct dependency"
```

---

## Task 7: Manual Integration Test

**Step 1: Create local config.yaml**

```bash
cp backend/config.example.yaml backend/config.yaml
```

Edit `backend/config.yaml` and set your `jwt.secret` value.

**Step 2: Run the server**

```bash
cd backend && go run ./cmd/server
```

**Step 3: Verify logging**

Expected output includes:
- "geoip: database initialized successfully" (if GeoIP path set)
- OR "geoip initialization failed - location lookups disabled" (if no path)
- Server starts normally on configured port

**Step 4: Test with GeoIP database (optional)**

If you have a MaxMind GeoIP2 City database:
1. Set `geoip.path` in `config.yaml` to the `.mmdb` file path
2. Restart server
3. Click a shortened link
4. Check database/stats for country/city data

---

## Summary

This plan:
1. Adds YAML config support while maintaining backward compatibility with env vars
2. Env vars override YAML values (useful for containerized deployments)
3. Fixes the GeoIP initialization bug - `InitGeoIP()` now called in main.go
4. Adds proper logging for GeoIP status
5. Keeps `config.yaml` out of version control (contains secrets)

After implementation, location data should display correctly once a valid GeoIP database is configured.
