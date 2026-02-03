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
