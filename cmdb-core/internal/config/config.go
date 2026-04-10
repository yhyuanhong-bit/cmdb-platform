package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the cmdb-core service.
type Config struct {
	Port        int
	DatabaseURL string
	RedisURL    string
	NatsURL     string
	JWTSecret   string
	DeployMode  string
	TenantID    string
	LogLevel    string
	MCPEnabled   bool
	MCPPort      int
	WSEnabled    bool
	OTELEndpoint string
	MCPApiKey    string
}

// Load reads configuration from environment variables with sensible defaults.
// In edge mode, TenantID is required.
func Load() (*Config, error) {
	port := 8080
	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		port = p
	}

	cfg := &Config{
		Port:        port,
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://localhost:5432/cmdb?sslmode=disable"),
		RedisURL:    envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		NatsURL:     envOrDefault("NATS_URL", "nats://localhost:4222"),
		JWTSecret:   envOrDefault("JWT_SECRET", "dev-secret-change-me"),
		DeployMode:  envOrDefault("DEPLOY_MODE", "cloud"),
		TenantID:    os.Getenv("TENANT_ID"),
		LogLevel:    envOrDefault("LOG_LEVEL", "info"),
	}

	cfg.MCPEnabled = envOrDefault("MCP_ENABLED", "true") == "true"
	mcpPort := 3001
	if v := os.Getenv("MCP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			mcpPort = p
		}
	}
	cfg.MCPPort = mcpPort
	cfg.WSEnabled = envOrDefault("WS_ENABLED", "true") == "true"
	cfg.OTELEndpoint = os.Getenv("OTEL_ENDPOINT")
	cfg.MCPApiKey = os.Getenv("MCP_API_KEY")

	if cfg.DeployMode == "edge" && cfg.TenantID == "" {
		return nil, fmt.Errorf("TENANT_ID is required in edge deploy mode")
	}

	// Reject insecure JWT secret in non-development mode
	if cfg.JWTSecret == "dev-secret-change-me" && cfg.DeployMode != "edge" {
		if os.Getenv("DEPLOY_MODE") != "" {
			// Explicit non-dev deploy mode — reject default secret
			return nil, fmt.Errorf("JWT_SECRET must be set to a secure value (minimum 32 characters) in %s mode", cfg.DeployMode)
		}
	}
	if len(cfg.JWTSecret) < 32 {
		fmt.Fprintf(os.Stderr, "WARNING: JWT_SECRET is shorter than 32 characters — this is insecure for production\n")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
