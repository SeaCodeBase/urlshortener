// backend/internal/config/config.go
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port              string   `yaml:"port"`          // Legacy, kept for backwards compat
	RedirectPort      string   `yaml:"redirect_port"` // Port for redirect server
	APIPort           string   `yaml:"api_port"`      // Port for API server
	AllowRegistration *bool    `yaml:"allow_registration"`
	AllowOrigins      []string `yaml:"allow_origins"`
	// TrustedProxies lists the IPs/CIDRs of the reverse proxy in front of
	// the redirect server whose X-Forwarded-For header is believed. Empty
	// (the default): the header is ignored and the TCP peer is the client.
	TrustedProxies []string `yaml:"trusted_proxies"`
	// APITrustedProxies is the same for the API server, whose public
	// page-view endpoint is reached through the Next.js proxy. Absent
	// defaults to loopback; an explicit [] trusts nobody. Read it with
	// APIProxies.
	APITrustedProxies *[]string `yaml:"api_trusted_proxies"`
}

// defaultAPITrustedProxies is the co-located Next.js server in development.
var defaultAPITrustedProxies = []string{"127.0.0.1", "::1"}

// APIProxies returns the API server's trusted proxies (after defaults).
func (s ServerConfig) APIProxies() []string {
	if s.APITrustedProxies == nil {
		return append([]string(nil), defaultAPITrustedProxies...)
	}
	return *s.APITrustedProxies
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	// MigrationsPath is the directory holding NNN_*.sql files applied on startup.
	MigrationsPath string `yaml:"migrations_path"`
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
}

// JWTConfig holds JWT authentication configuration
type JWTConfig struct {
	Secret string `yaml:"secret"`
}

// WebAuthnConfig holds WebAuthn/passkey configuration
type WebAuthnConfig struct {
	RPID     string `yaml:"rp_id"`
	RPOrigin string `yaml:"rp_origin"`
}

// URLsConfig holds URL-related configuration
type URLsConfig struct {
	// BaseURL is the public URL of the redirect server (short links).
	BaseURL string `yaml:"base_url"`
	// FrontendURL is the public URL of the web UI (used for links back to the app).
	FrontendURL string `yaml:"frontend_url"`
	// RedirectHost is the hostname custom domains should CNAME to.
	// Defaults to the host of BaseURL.
	RedirectHost string `yaml:"redirect_host"`
	// RedirectIP is the IP custom apex domains should point an A record at.
	// Empty means "not advertised".
	RedirectIP string `yaml:"redirect_ip"`
}

// UIConfig holds settings that affect what the web UI renders.
type UIConfig struct {
	// FaviconService enables favicons via Google's favicon service.
	// nil (unset) defaults to true; set false for offline installs.
	FaviconService *bool `yaml:"favicon_service"`
}

// UploadsConfig holds settings for user-uploaded files.
type UploadsConfig struct {
	// Path is the directory where uploaded files are stored.
	Path string `yaml:"path"`
}

// GeoIPConfig holds GeoIP database configuration
type GeoIPConfig struct {
	Path string `yaml:"path"`
}

// WebhookWorkerConfig tunes the webhook dispatcher.
type WebhookWorkerConfig struct {
	// PollEvery is how often due deliveries are picked up (default 2s).
	PollEvery time.Duration `yaml:"poll_every"`
}

// BulkWorkerConfig tunes the bulk-upload worker.
type BulkWorkerConfig struct {
	// PollEvery is how often queued uploads are picked up (default 5s).
	PollEvery time.Duration `yaml:"poll_every"`
	// BatchSize is how many rows are processed between progress updates
	// (default 50).
	BatchSize int `yaml:"batch_size"`
}

// WorkersConfig holds the background workers' settings.
type WorkersConfig struct {
	Webhooks WebhookWorkerConfig `yaml:"webhooks"`
	Bulk     BulkWorkerConfig    `yaml:"bulk"`
}

// Config is the main application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
	WebAuthn WebAuthnConfig `yaml:"webauthn"`
	URLs     URLsConfig     `yaml:"urls"`
	GeoIP    GeoIPConfig    `yaml:"geoip"`
	UI       UIConfig       `yaml:"ui"`
	Uploads  UploadsConfig  `yaml:"uploads"`
	Workers  WorkersConfig  `yaml:"workers"`
}

// LoadFromYAML loads configuration from a YAML file. Parsing is strict: an
// unknown (e.g. misspelled) key fails the load instead of silently falling
// back to the default.
func LoadFromYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg, err := decodeStrict(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	applyEnv(cfg)

	applyDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// decodeStrict decodes YAML into a Config, rejecting keys that match no
// field. An empty document yields a zero Config.
func decodeStrict(data []byte) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return &cfg, nil
}

// Load loads configuration from config.yaml
func Load() (*Config, error) {
	yamlPaths := []string{"config.yaml", "../config.yaml", "../../config.yaml"}
	for _, path := range yamlPaths {
		if _, err := os.Stat(path); err == nil {
			return LoadFromYAML(path)
		}
	}

	return nil, errors.New("config.yaml not found, please copy config.example.yaml to config.yaml")
}

// Environment variables that override the config file when non-empty.
const (
	EnvJWTSecret = "JWT_SECRET" // overrides jwt.secret
	EnvBaseURL   = "BASE_URL"   // overrides urls.base_url
)

// applyEnv applies the environment overrides. It runs before applyDefaults,
// so values derived from urls.base_url (redirect_host) follow BASE_URL.
func applyEnv(cfg *Config) {
	if v := os.Getenv(EnvJWTSecret); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv(EnvBaseURL); v != "" {
		cfg.URLs.BaseURL = v
	}
}

// placeholderSecrets are the publicly known secrets shipped in the
// repository (config.docker.yaml, .env.example).
var placeholderSecrets = map[string]bool{
	"dev-jwt-secret-change-in-production": true,
	"your_jwt_secret_key":                 true,
}

// minSecretLen is the length below which the JWT secret is reported as weak.
const minSecretLen = 16

// SecretWarning returns a warning to log at startup when secret is a
// placeholder from the repository or too short, and "" otherwise. The
// secret signs session JWTs and salts click/page-view IP hashes.
func SecretWarning(secret string) string {
	if placeholderSecrets[secret] {
		return "jwt.secret is a placeholder published in the repository: anyone can forge session tokens; set JWT_SECRET (or jwt.secret) to a long random string"
	}
	if len(secret) < minSecretLen {
		return fmt.Sprintf("jwt.secret is shorter than %d bytes; set JWT_SECRET (or jwt.secret) to a long random string", minSecretLen)
	}
	return ""
}

// applyDefaults sets default values for empty fields
func applyDefaults(cfg *Config) {
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Server.RedirectPort == "" {
		cfg.Server.RedirectPort = "8080"
	}
	if cfg.Server.APIPort == "" {
		cfg.Server.APIPort = "8081"
	}
	if cfg.Server.AllowRegistration == nil {
		allowReg := true
		cfg.Server.AllowRegistration = &allowReg
	}
	if cfg.Server.APITrustedProxies == nil {
		proxies := append([]string(nil), defaultAPITrustedProxies...)
		cfg.Server.APITrustedProxies = &proxies
	}
	if len(cfg.Server.AllowOrigins) == 0 {
		cfg.Server.AllowOrigins = []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		}
	}
	if cfg.Database.Host == "" {
		cfg.Database.Host = "localhost"
	}
	if cfg.Database.Port == "" {
		cfg.Database.Port = "3306"
	}
	if cfg.Database.User == "" {
		cfg.Database.User = "urlshortener"
	}
	if cfg.Database.Name == "" {
		cfg.Database.Name = "urlshortener"
	}
	if cfg.Database.MigrationsPath == "" {
		cfg.Database.MigrationsPath = "migrations"
	}
	if cfg.Redis.Host == "" {
		cfg.Redis.Host = "localhost"
	}
	if cfg.Redis.Port == "" {
		cfg.Redis.Port = "6379"
	}
	if cfg.URLs.BaseURL == "" {
		cfg.URLs.BaseURL = "http://localhost:8080"
	}
	if cfg.URLs.FrontendURL == "" {
		cfg.URLs.FrontendURL = "http://localhost:3000"
	}
	if cfg.URLs.RedirectHost == "" {
		if u, err := url.Parse(cfg.URLs.BaseURL); err == nil {
			cfg.URLs.RedirectHost = u.Hostname()
		}
	}
	if cfg.UI.FaviconService == nil {
		enabled := true
		cfg.UI.FaviconService = &enabled
	}
	if cfg.Uploads.Path == "" {
		cfg.Uploads.Path = "data/uploads"
	}
	if cfg.Workers.Webhooks.PollEvery <= 0 {
		cfg.Workers.Webhooks.PollEvery = 2 * time.Second
	}
	if cfg.Workers.Bulk.PollEvery <= 0 {
		cfg.Workers.Bulk.PollEvery = 5 * time.Second
	}
	if cfg.Workers.Bulk.BatchSize <= 0 {
		cfg.Workers.Bulk.BatchSize = 50
	}
	if cfg.WebAuthn.RPID == "" {
		cfg.WebAuthn.RPID = "localhost"
	}
	if cfg.WebAuthn.RPOrigin == "" {
		cfg.WebAuthn.RPOrigin = "http://localhost:3000"
	}
}

// validate checks for required configuration values
func validate(cfg *Config) error {
	if cfg.JWT.Secret == "" {
		return errors.New("jwt.secret is required: set it in config.yaml or via the JWT_SECRET environment variable")
	}
	if err := validateProxies("server.trusted_proxies", cfg.Server.TrustedProxies); err != nil {
		return err
	}
	return validateProxies("server.api_trusted_proxies", cfg.Server.APIProxies())
}

// validateProxies accepts what gin.Engine.SetTrustedProxies accepts: an IP,
// or a CIDR when the entry contains "/".
func validateProxies(key string, proxies []string) error {
	for _, p := range proxies {
		if strings.Contains(p, "/") {
			if _, _, err := net.ParseCIDR(p); err != nil {
				return fmt.Errorf("%s: invalid CIDR %q", key, p)
			}
		} else if net.ParseIP(p) == nil {
			return fmt.Errorf("%s: invalid IP %q", key, p)
		}
	}
	return nil
}
