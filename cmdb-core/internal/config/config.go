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
	DeployMode  string
	TenantID    string
	LogLevel    string
	MCPEnabled   bool
	MCPPort      int
	WSEnabled    bool
	OTELEndpoint string
	MCPApiKey            string
	CarbonEmissionFactor  float64
	SyncEnabled           bool
	SyncSnapshotBatchSize int
	EdgeNodeID            string
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
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"),
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
	cfg.CarbonEmissionFactor = envOrDefaultFloat("CARBON_EMISSION_FACTOR", 0.0005)
	cfg.SyncEnabled = envOrDefault("SYNC_ENABLED", "true") == "true"
	cfg.SyncSnapshotBatchSize = envOrDefaultInt("SYNC_SNAPSHOT_BATCH_SIZE", 500)
	cfg.EdgeNodeID = envOrDefault("EDGE_NODE_ID", "")

	if cfg.DeployMode == "edge" && cfg.TenantID == "" {
		return nil, fmt.Errorf("TENANT_ID is required in edge deploy mode")
	}

	// Enforce secure credentials in production (cloud) mode.
	// Edge mode allows defaults for development convenience.
	if cfg.JWTSecret == "dev-secret-change-me" {
		if cfg.DeployMode == "cloud" {
			return nil, fmt.Errorf("JWT_SECRET must be changed from default for production deployment")
		}
		// Edge mode allows default for development convenience
		fmt.Println("WARNING: JWT_SECRET is shorter than 32 characters — this is insecure for production")
	}
	if cfg.DeployMode == "cloud" && strings.Contains(cfg.DatabaseURL, "changeme") {
		return nil, fmt.Errorf("DATABASE_URL contains default password 'changeme' — set a secure password for production")
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
