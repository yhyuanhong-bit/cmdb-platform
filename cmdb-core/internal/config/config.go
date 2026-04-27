package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the cmdb-core service.
type Config struct {
	Port        int
	DatabaseURL string
	RedisURL    string
	NatsURL     string
	JWTSecret   string
	TenantID    string
	LogLevel    string
	MCPEnabled   bool
	MCPPort      int
	WSEnabled    bool
	OTELEndpoint string
	MCPApiKey            string
	CarbonEmissionFactor  float64
	RateLimitEnabled      bool
	RateLimitRPS          float64
	RateLimitBurst        int

	// IntegrationAllowedOutboundHosts is an admin-configured allowlist of
	// hostnames (exact match, case-insensitive) that bypass SSRF protection
	// for outbound integration, webhook, and adapter HTTP calls. Comma-
	// separated in CMDB_INTEGRATION_ALLOWED_HOSTS. Empty by default.
	IntegrationAllowedOutboundHosts []string
}

// Load reads configuration from environment variables with sensible defaults.
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
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"),
		RedisURL:    envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		NatsURL:     envOrDefault("NATS_URL", "nats://localhost:4222"),
		JWTSecret:   envOrDefault("JWT_SECRET", "dev-secret-change-me"),
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
	cfg.CarbonEmissionFactor = envOrDefaultFloat("CARBON_EMISSION_FACTOR", 0.0005)
	cfg.RateLimitEnabled = envOrDefault("RATE_LIMIT_ENABLED", "true") == "true"
	cfg.RateLimitRPS = envOrDefaultFloat("RATE_LIMIT_RPS", 100)
	cfg.RateLimitBurst = envOrDefaultInt("RATE_LIMIT_BURST", 200)

	// SSRF allowlist for integration outbound calls. Accepts a comma-
	// separated list of hostnames. Blank entries are discarded.
	if raw := os.Getenv("CMDB_INTEGRATION_ALLOWED_HOSTS"); raw != "" {
		for _, h := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(h); trimmed != "" {
				cfg.IntegrationAllowedOutboundHosts = append(cfg.IntegrationAllowedOutboundHosts, trimmed)
			}
		}
	}

	// Treat default-password DB URL as the dev signal. In dev (default
	// JWT secret + default DB password) we warn and continue so local
	// `docker-compose up` works out of the box. As soon as the operator
	// flips to a real DB URL the secret check turns fatal — this prevents
	// a forgotten "dev-secret-change-me" from reaching prod.
	devDB := strings.Contains(cfg.DatabaseURL, "changeme")
	if cfg.JWTSecret == "dev-secret-change-me" || len(cfg.JWTSecret) < 32 {
		if !devDB {
			return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters and not the default value for production deployment")
		}
		fmt.Fprintf(os.Stderr, "WARNING: JWT_SECRET is insecure (default or shorter than 32 chars) — set a strong secret before production use\n")
	}

	return cfg, nil
}

func envOrDefaultFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envOrDefaultInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
