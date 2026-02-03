// backend/internal/config/config.go
package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port string `yaml:"port"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
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
	BaseURL string `yaml:"base_url"`
}

// GeoIPConfig holds GeoIP database configuration
type GeoIPConfig struct {
	Path string `yaml:"path"`
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
}

// LoadFromYAML loads configuration from a YAML file with env var overrides
func LoadFromYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(&cfg)
	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
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

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SERVER_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		cfg.Database.Port = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		cfg.Redis.Port = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		cfg.URLs.BaseURL = v
	}
	if v := os.Getenv("RP_ID"); v != "" {
		cfg.WebAuthn.RPID = v
	}
	if v := os.Getenv("RP_ORIGIN"); v != "" {
		cfg.WebAuthn.RPOrigin = v
	}
	if v := os.Getenv("GEOIP_PATH"); v != "" {
		cfg.GeoIP.Path = v
	}
}

// applyDefaults sets default values for empty fields
func applyDefaults(cfg *Config) {
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
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
	if cfg.Redis.Host == "" {
		cfg.Redis.Host = "localhost"
	}
	if cfg.Redis.Port == "" {
		cfg.Redis.Port = "6379"
	}
	if cfg.URLs.BaseURL == "" {
		cfg.URLs.BaseURL = "http://localhost:8080"
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
		return errors.New("jwt.secret is required in config.yaml")
	}
	return nil
}
