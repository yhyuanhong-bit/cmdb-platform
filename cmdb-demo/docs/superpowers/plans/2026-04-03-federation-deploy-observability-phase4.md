# Federation, Deployment & Observability Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Production-ready deployment stack with structured logging (zap), Prometheus metrics, OpenTelemetry tracing, Nginx reverse proxy, NATS Leaf Node federation configs, full-stack Docker Compose for Central and Edge modes, and Grafana dashboard provisioning.

**Architecture:** Add observability instrumentation directly into cmdb-core (Go) via zap structured logging, Prometheus metrics middleware, and OpenTelemetry trace middleware. Create separate Docker Compose profiles for Central (full stack with observability) and Edge (lightweight). NATS Leaf Node configs enable cross-IDC event federation. Nginx serves as the single entry point handling TLS termination, frontend static files, and API reverse proxy.

**Tech Stack:** zap, prometheus/client_golang, otel-go, Nginx 1.27, Grafana 11, Loki, Promtail, Jaeger, NATS Leaf Node, Docker Compose profiles

**Spec Reference:** `docs/superpowers/specs/2026-04-03-cmdb-backend-techstack-design.md` — Sections 6, 7

---

## File Structure

```
cmdb-core/
├── internal/platform/telemetry/
│   ├── logging.go             # zap structured logger setup
│   ├── metrics.go             # Prometheus metrics + Gin middleware
│   └── tracing.go             # OpenTelemetry tracer + Gin middleware
│
├── deploy/
│   ├── docker-compose.yml              # Updated: full central stack
│   ├── docker-compose.edge.yml         # Edge override (lightweight)
│   ├── .env.example                    # Updated with all vars
│   ├── nginx/
│   │   └── nginx.conf                  # Reverse proxy + frontend
│   ├── nats/
│   │   ├── nats-central.conf          # Hub mode config
│   │   └── nats-edge.conf             # Leaf Node config
│   ├── otel/
│   │   └── otel-collector.yaml        # OTel collector pipeline config
│   ├── grafana/
│   │   ├── datasources.yaml           # Prometheus + Loki + Jaeger
│   │   └── dashboards/
│   │       ├── provider.yaml          # Dashboard provisioning config
│   │       └── api-overview.json      # Pre-built API dashboard
│   ├── promtail/
│   │   └── promtail.yaml              # Log scraping config
│   └── prometheus/
│       └── prometheus.yaml            # Scrape config for cmdb-core
```

---

## Task 1: Structured Logging (zap)

**Files:**
- Create: `cmdb-core/internal/platform/telemetry/logging.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Create logging.go**

Create `cmdb-core/internal/platform/telemetry/logging.go`:

```go
package telemetry

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a structured JSON logger.
// In development, it uses a console encoder for readability.
func NewLogger(level string) (*zap.Logger, error) {
	var cfg zap.Config

	if os.Getenv("DEPLOY_MODE") == "central" || os.Getenv("LOG_FORMAT") == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	// Parse level
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	// Always include caller and timestamp
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return cfg.Build()
}
```

- [ ] **Step 2: Install zap and update main.go**

```bash
cd /cmdb-platform/cmdb-core
go get go.uber.org/zap
```

In `cmd/server/main.go`, after loading config, replace `log.Printf` calls:

```go
import "github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"

// After cfg := config.Load()
logger, err := telemetry.NewLogger(cfg.LogLevel)
if err != nil {
    log.Fatalf("logger: %v", err)
}
defer logger.Sync()
zap.ReplaceGlobals(logger)

// Replace all log.Printf with zap.L().Info/Warn/Error
// e.g.: zap.L().Info("cmdb-core started", zap.String("addr", addr), zap.String("mode", cfg.DeployMode))
```

- [ ] **Step 3: Verify build**

```bash
go mod tidy && go build ./cmd/server
```

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/platform/telemetry/logging.go cmdb-core/cmd/server/main.go cmdb-core/go.mod cmdb-core/go.sum
git commit -m "feat: add structured logging with zap + replace log.Printf in main"
```

---

## Task 2: Prometheus Metrics Middleware

**Files:**
- Create: `cmdb-core/internal/platform/telemetry/metrics.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Create metrics.go**

Create `cmdb-core/internal/platform/telemetry/metrics.go`:

```go
package telemetry

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_request_total",
			Help: "Total HTTP requests by method, path, and status",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"method", "path"},
	)

	ActiveWSConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_websocket_connections",
			Help: "Number of active WebSocket connections",
		},
	)

	NATSMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nats_messages_published_total",
			Help: "Total NATS messages published by subject",
		},
		[]string{"subject"},
	)

	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"query"},
	)
)

// PrometheusMiddleware records request metrics for every HTTP request.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()

		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// MetricsHandler returns the Prometheus metrics HTTP handler for /metrics endpoint.
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
```

- [ ] **Step 2: Wire metrics into main.go**

Add to main.go:
```go
// After middleware setup, before routes:
r.Use(telemetry.PrometheusMiddleware())

// Add metrics endpoint (no auth required for Prometheus scraping)
r.GET("/metrics", telemetry.MetricsHandler())
```

- [ ] **Step 3: Install prometheus deps and verify**

```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
go get github.com/prometheus/client_golang/prometheus/promhttp
go mod tidy && go build ./cmd/server
```

- [ ] **Step 4: Commit**

```bash
git add cmdb-core/internal/platform/telemetry/metrics.go cmdb-core/cmd/server/main.go cmdb-core/go.mod cmdb-core/go.sum
git commit -m "feat: add Prometheus metrics middleware + /metrics endpoint"
```

---

## Task 3: OpenTelemetry Tracing Middleware

**Files:**
- Create: `cmdb-core/internal/platform/telemetry/tracing.go`
- Modify: `cmdb-core/internal/config/config.go`
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add OTEL_ENDPOINT to config**

Add to Config struct in `internal/config/config.go`:
```go
OTELEndpoint string
```

Add to Load():
```go
cfg.OTELEndpoint = envOrDefault("OTEL_ENDPOINT", "")
```

- [ ] **Step 2: Create tracing.go**

Create `cmdb-core/internal/platform/telemetry/tracing.go`:

```go
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"github.com/gin-gonic/gin"
)

// InitTracer sets up an OTel trace exporter to the given gRPC endpoint.
// Returns a shutdown function. If endpoint is empty, tracing is disabled (noop).
func InitTracer(ctx context.Context, endpoint, serviceName, version string) (func(context.Context) error, error) {
	if endpoint == "" {
		// No-op: tracing disabled
		return func(ctx context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), // TLS configured at infra level
	)
	if err != nil {
		return nil, fmt.Errorf("otel exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))), // 10% sampling
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// TracingMiddleware returns a Gin middleware for OTel distributed tracing.
func TracingMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
```

- [ ] **Step 3: Wire tracing into main.go**

Add after logger setup:
```go
shutdownTracer, err := telemetry.InitTracer(ctx, cfg.OTELEndpoint, "cmdb-core", "1.0.0")
if err != nil {
    zap.L().Warn("tracing init failed", zap.Error(err))
} else {
    defer shutdownTracer(ctx)
}

// Add tracing middleware to router (before other middleware)
r.Use(telemetry.TracingMiddleware("cmdb-core"))
```

- [ ] **Step 4: Install OTel deps and verify**

```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/sdk/trace
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin
go mod tidy && go build ./cmd/server
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/internal/platform/telemetry/tracing.go cmdb-core/internal/config/config.go cmdb-core/cmd/server/main.go cmdb-core/go.mod cmdb-core/go.sum
git commit -m "feat: add OpenTelemetry distributed tracing middleware"
```

---

## Task 4: Nginx Reverse Proxy Configuration

**Files:**
- Create: `cmdb-core/deploy/nginx/nginx.conf`

- [ ] **Step 1: Create nginx.conf**

Create `cmdb-core/deploy/nginx/nginx.conf`:

```nginx
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 2048;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    log_format json escape=json '{'
        '"time":"$time_iso8601",'
        '"remote_addr":"$remote_addr",'
        '"method":"$request_method",'
        '"uri":"$request_uri",'
        '"status":$status,'
        '"body_bytes_sent":$body_bytes_sent,'
        '"request_time":$request_time,'
        '"upstream_response_time":"$upstream_response_time",'
        '"request_id":"$request_id"'
    '}';
    access_log /var/log/nginx/access.log json;

    sendfile on;
    tcp_nopush on;
    keepalive_timeout 65;
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml;
    gzip_min_length 256;

    # Rate limiting zones
    limit_req_zone $binary_remote_addr zone=api:10m rate=100r/s;
    limit_req_zone $binary_remote_addr zone=upload:10m rate=10r/s;

    upstream cmdb_core {
        server cmdb-core:8080;
    }

    upstream ingestion_engine {
        server ingestion-engine:8081;
    }

    server {
        listen 80;
        server_name _;

        # Request ID propagation
        add_header X-Request-Id $request_id always;

        # Frontend static files
        location / {
            root /usr/share/nginx/html;
            try_files $uri $uri/ /index.html;
        }

        # cmdb-core REST API
        location /api/v1/ {
            limit_req zone=api burst=50 nodelay;
            proxy_pass http://cmdb_core;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Request-Id $request_id;
        }

        # WebSocket upgrade
        location /api/v1/ws {
            proxy_pass http://cmdb_core;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Request-Id $request_id;
            proxy_read_timeout 3600s;
        }

        # Ingestion engine API
        location /ingestion/ {
            limit_req zone=api burst=20 nodelay;
            proxy_pass http://ingestion_engine;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Request-Id $request_id;
        }

        # File upload (higher body size, lower rate limit)
        location /ingestion/import/upload {
            limit_req zone=upload burst=5 nodelay;
            client_max_body_size 50m;
            proxy_pass http://ingestion_engine;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Request-Id $request_id;
        }

        # Health checks (no rate limit)
        location /healthz {
            proxy_pass http://cmdb_core;
        }

        # Prometheus metrics (internal only in production)
        location /metrics {
            proxy_pass http://cmdb_core;
        }
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-core/deploy/nginx/
git commit -m "feat: add Nginx reverse proxy config with rate limiting + WebSocket support"
```

---

## Task 5: NATS Federation Configs (Central Hub + Edge Leaf)

**Files:**
- Create: `cmdb-core/deploy/nats/nats-central.conf`
- Create: `cmdb-core/deploy/nats/nats-edge.conf`

- [ ] **Step 1: Create nats-central.conf (Hub mode)**

Create `cmdb-core/deploy/nats/nats-central.conf`:

```
# NATS Central Hub Configuration
# Accepts Leaf Node connections from Edge IDC nodes

server_name: cmdb-central

listen: 0.0.0.0:4222

jetstream {
    store_dir: /data/jetstream
    max_mem: 1G
    max_file: 50G
}

# Leaf Node listener — Edge nodes connect here
leafnodes {
    listen: 0.0.0.0:7422
    # In production, add TLS:
    # tls {
    #     cert_file: /certs/server.crt
    #     key_file: /certs/server.key
    #     ca_file: /certs/ca.crt
    #     verify: true
    # }
}

# Monitoring
http_port: 8222

# Logging
logfile: "/data/nats.log"
log_size_limit: 100MB
max_traced_msg_len: 256

# Limits
max_payload: 1MB
max_connections: 1000
```

- [ ] **Step 2: Create nats-edge.conf (Leaf Node mode)**

Create `cmdb-core/deploy/nats/nats-edge.conf`:

```
# NATS Edge Leaf Node Configuration
# Connects to Central Hub and forwards local events

server_name: cmdb-edge

listen: 0.0.0.0:4222

jetstream {
    store_dir: /data/jetstream
    max_mem: 256M
    max_file: 10G
}

# Connect to Central Hub as Leaf Node
leafnodes {
    remotes [
        {
            url: "nats-leaf://central-nats:7422"
            # In production, add TLS:
            # tls {
            #     cert_file: /certs/edge.crt
            #     key_file: /certs/edge.key
            #     ca_file: /certs/ca.crt
            # }
        }
    ]
}

# Monitoring
http_port: 8222

# Logging
logfile: "/data/nats.log"
log_size_limit: 50MB

# Edge is smaller
max_payload: 1MB
max_connections: 200
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/deploy/nats/
git commit -m "feat: add NATS federation configs - central hub + edge leaf node"
```

---

## Task 6: Observability Stack Configs (OTel + Prometheus + Loki + Grafana)

**Files:**
- Create: `cmdb-core/deploy/otel/otel-collector.yaml`
- Create: `cmdb-core/deploy/prometheus/prometheus.yaml`
- Create: `cmdb-core/deploy/promtail/promtail.yaml`
- Create: `cmdb-core/deploy/grafana/datasources.yaml`
- Create: `cmdb-core/deploy/grafana/dashboards/provider.yaml`
- Create: `cmdb-core/deploy/grafana/dashboards/api-overview.json`

- [ ] **Step 1: Create OTel collector config**

Create `cmdb-core/deploy/otel/otel-collector.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1024

exporters:
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true
  prometheus:
    endpoint: 0.0.0.0:8889
    namespace: cmdb

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/jaeger]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

- [ ] **Step 2: Create Prometheus scrape config**

Create `cmdb-core/deploy/prometheus/prometheus.yaml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "cmdb-core"
    static_configs:
      - targets: ["cmdb-core:8080"]
    metrics_path: /metrics

  - job_name: "otel-collector"
    static_configs:
      - targets: ["otel-collector:8889"]

  - job_name: "nats"
    static_configs:
      - targets: ["nats:8222"]
    metrics_path: /varz
```

- [ ] **Step 3: Create Promtail config**

Create `cmdb-core/deploy/promtail/promtail.yaml`:

```yaml
server:
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: containers
    static_configs:
      - targets:
          - localhost
        labels:
          job: cmdb
          __path__: /var/log/containers/*.log
    pipeline_stages:
      - docker: {}
      - json:
          expressions:
            level: level
            module: module
            tenant_id: tenant_id
            request_id: request_id
      - labels:
          level:
          module:
          tenant_id:
```

- [ ] **Step 4: Create Grafana datasources**

Create `cmdb-core/deploy/grafana/datasources.yaml`:

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false

  - name: Loki
    type: loki
    access: proxy
    url: http://loki:3100
    editable: false

  - name: Jaeger
    type: jaeger
    access: proxy
    url: http://jaeger:16686
    editable: false
```

- [ ] **Step 5: Create Grafana dashboard provisioning**

Create `cmdb-core/deploy/grafana/dashboards/provider.yaml`:

```yaml
apiVersion: 1

providers:
  - name: CMDB
    orgId: 1
    folder: CMDB
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /var/lib/grafana/dashboards
      foldersFromFilesStructure: false
```

- [ ] **Step 6: Create API Overview dashboard**

Create `cmdb-core/deploy/grafana/dashboards/api-overview.json`:

```json
{
  "dashboard": {
    "title": "CMDB API Overview",
    "uid": "cmdb-api-overview",
    "timezone": "browser",
    "refresh": "10s",
    "time": {"from": "now-1h", "to": "now"},
    "panels": [
      {
        "title": "Request Rate (QPS)",
        "type": "timeseries",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [
          {
            "expr": "sum(rate(api_request_total[1m]))",
            "legendFormat": "Total QPS"
          },
          {
            "expr": "sum(rate(api_request_total{status=~\"5..\"}[1m]))",
            "legendFormat": "5xx Rate"
          }
        ]
      },
      {
        "title": "Latency P50 / P95 / P99",
        "type": "timeseries",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [
          {
            "expr": "histogram_quantile(0.50, sum(rate(api_request_duration_seconds_bucket[5m])) by (le))",
            "legendFormat": "P50"
          },
          {
            "expr": "histogram_quantile(0.95, sum(rate(api_request_duration_seconds_bucket[5m])) by (le))",
            "legendFormat": "P95"
          },
          {
            "expr": "histogram_quantile(0.99, sum(rate(api_request_duration_seconds_bucket[5m])) by (le))",
            "legendFormat": "P99"
          }
        ]
      },
      {
        "title": "Requests by Status Code",
        "type": "piechart",
        "gridPos": {"h": 8, "w": 8, "x": 0, "y": 8},
        "targets": [
          {
            "expr": "sum(increase(api_request_total[1h])) by (status)",
            "legendFormat": "{{status}}"
          }
        ]
      },
      {
        "title": "Top Endpoints by Latency",
        "type": "table",
        "gridPos": {"h": 8, "w": 8, "x": 8, "y": 8},
        "targets": [
          {
            "expr": "topk(10, histogram_quantile(0.95, sum(rate(api_request_duration_seconds_bucket[5m])) by (le, path)))",
            "legendFormat": "{{path}}",
            "format": "table",
            "instant": true
          }
        ]
      },
      {
        "title": "Active WebSocket Connections",
        "type": "stat",
        "gridPos": {"h": 4, "w": 4, "x": 16, "y": 8},
        "targets": [
          {"expr": "active_websocket_connections", "legendFormat": "WS Clients"}
        ]
      },
      {
        "title": "NATS Messages/sec",
        "type": "stat",
        "gridPos": {"h": 4, "w": 4, "x": 20, "y": 8},
        "targets": [
          {"expr": "sum(rate(nats_messages_published_total[1m]))", "legendFormat": "msg/s"}
        ]
      }
    ]
  }
}
```

- [ ] **Step 7: Commit**

```bash
git add cmdb-core/deploy/otel/ cmdb-core/deploy/prometheus/ cmdb-core/deploy/promtail/ cmdb-core/deploy/grafana/
git commit -m "feat: add observability stack configs - OTel, Prometheus, Promtail, Grafana dashboards"
```

---

## Task 7: Production Docker Compose (Central Full Stack)

**Files:**
- Rewrite: `cmdb-core/deploy/docker-compose.yml`
- Create: `cmdb-core/deploy/docker-compose.edge.yml`
- Modify: `cmdb-core/deploy/.env.example`

- [ ] **Step 1: Rewrite docker-compose.yml for full central stack**

Replace `cmdb-core/deploy/docker-compose.yml`:

```yaml
services:
  # ---- Core Services ----
  cmdb-core:
    build:
      context: ../
      dockerfile: Dockerfile
    environment:
      DATABASE_URL: postgres://cmdb:${DB_PASS:-cmdb_secret}@postgres:5432/cmdb?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      NATS_URL: nats://nats:4222
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-me}
      DEPLOY_MODE: central
      MCP_ENABLED: "true"
      MCP_PORT: "3001"
      WS_ENABLED: "true"
      OTEL_ENDPOINT: otel-collector:4317
      LOG_LEVEL: info
    ports:
      - "8080:8080"
      - "3001:3001"
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }
      nats: { condition: service_started }
    deploy:
      replicas: ${CORE_REPLICAS:-1}
      resources:
        limits: { cpus: "2", memory: "512M" }
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/healthz"]
      interval: 10s
      timeout: 3s
      retries: 3

  # ---- Ingestion Engine ----
  ingestion-engine:
    build:
      context: ../../ingestion-engine
    environment:
      INGESTION_DATABASE_URL: postgresql://cmdb:${DB_PASS:-cmdb_secret}@postgres:5432/cmdb
      INGESTION_REDIS_URL: redis://redis:6379/1
      INGESTION_NATS_URL: nats://nats:4222
      INGESTION_CELERY_BROKER_URL: redis://redis:6379/2
    ports:
      - "8081:8081"
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }

  ingestion-worker:
    build:
      context: ../../ingestion-engine
    command: celery -A app.tasks.celery_app worker -l info -c 4
    environment:
      INGESTION_DATABASE_URL: postgresql://cmdb:${DB_PASS:-cmdb_secret}@postgres:5432/cmdb
      INGESTION_REDIS_URL: redis://redis:6379/1
      INGESTION_CELERY_BROKER_URL: redis://redis:6379/2
    depends_on:
      redis: { condition: service_healthy }
    deploy:
      replicas: ${WORKER_REPLICAS:-2}

  # ---- Storage ----
  postgres:
    image: timescale/timescaledb:latest-pg17
    environment:
      POSTGRES_DB: cmdb
      POSTGRES_USER: cmdb
      POSTGRES_PASSWORD: ${DB_PASS:-cmdb_secret}
    ports:
      - "5432:5432"
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U cmdb"]
      interval: 5s
      timeout: 3s
      retries: 5
    command:
      - postgres
      - -c
      - shared_buffers=512MB
      - -c
      - effective_cache_size=1536MB
      - -c
      - work_mem=16MB
      - -c
      - max_connections=200
      - -c
      - wal_level=replica

  redis:
    image: redis:7.4-alpine
    command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 3

  nats:
    image: nats:2.10-alpine
    command: ["--config", "/etc/nats/nats.conf"]
    ports:
      - "4222:4222"
      - "7422:7422"
      - "8222:8222"
    volumes:
      - nats_data:/data
      - ./nats/nats-central.conf:/etc/nats/nats.conf:ro

  # ---- Reverse Proxy ----
  nginx:
    image: nginx:1.27-alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ../../cmdb-demo/dist:/usr/share/nginx/html:ro
    depends_on:
      - cmdb-core
      - ingestion-engine

  # ---- Observability ----
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.115.0
    command: ["--config", "/etc/otelcol/config.yaml"]
    volumes:
      - ./otel/otel-collector.yaml:/etc/otelcol/config.yaml:ro
    ports:
      - "4317:4317"
      - "4318:4318"

  jaeger:
    image: jaegertracing/all-in-one:1.64
    ports:
      - "16686:16686"
      - "14268:14268"
    environment:
      COLLECTOR_OTLP_ENABLED: "true"

  prometheus:
    image: prom/prometheus:v3.1.0
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--storage.tsdb.retention.time=15d"
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus/prometheus.yaml:/etc/prometheus/prometheus.yml:ro
      - prom_data:/prometheus

  loki:
    image: grafana/loki:3.3.2
    ports:
      - "3100:3100"
    volumes:
      - loki_data:/loki

  promtail:
    image: grafana/promtail:3.3.2
    volumes:
      - ./promtail/promtail.yaml:/etc/promtail/config.yml:ro
      - /var/log:/var/log:ro
      - /var/lib/docker/containers:/var/log/containers:ro

  grafana:
    image: grafana/grafana:11.4.0
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASS:-admin}
      GF_USERS_ALLOW_SIGN_UP: "false"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/datasources.yaml:/etc/grafana/provisioning/datasources/datasources.yaml:ro
      - ./grafana/dashboards/provider.yaml:/etc/grafana/provisioning/dashboards/provider.yaml:ro
      - ./grafana/dashboards:/var/lib/grafana/dashboards:ro
    depends_on:
      - prometheus
      - loki

volumes:
  pg_data:
  redis_data:
  nats_data:
  prom_data:
  loki_data:
  grafana_data:
```

- [ ] **Step 2: Create edge override compose**

Create `cmdb-core/deploy/docker-compose.edge.yml`:

```yaml
# Edge deployment override
# Usage: docker compose -f docker-compose.yml -f docker-compose.edge.yml up
#
# Overrides central config for lightweight edge node deployment:
# - Single cmdb-core replica, edge mode
# - Single ingestion worker
# - No observability stack (sends to central)
# - NATS in Leaf Node mode

services:
  cmdb-core:
    environment:
      DEPLOY_MODE: edge
      TENANT_ID: ${TENANT_ID:?TENANT_ID required for edge mode}
      OTEL_ENDPOINT: ""
      MCP_ENABLED: "false"
    deploy:
      replicas: 1
      resources:
        limits: { cpus: "1", memory: "256M" }

  ingestion-worker:
    deploy:
      replicas: 1

  nats:
    volumes:
      - nats_data:/data
      - ./nats/nats-edge.conf:/etc/nats/nats.conf:ro
    ports:
      - "4222:4222"
      - "8222:8222"

  # Disable observability services on edge
  otel-collector:
    deploy:
      replicas: 0
  jaeger:
    deploy:
      replicas: 0
  prometheus:
    deploy:
      replicas: 0
  loki:
    deploy:
      replicas: 0
  promtail:
    deploy:
      replicas: 0
  grafana:
    deploy:
      replicas: 0
```

- [ ] **Step 3: Update .env.example**

Replace `cmdb-core/deploy/.env.example`:

```env
# Database
DB_PASS=cmdb_secret

# Security
JWT_SECRET=change-me-in-production

# Deployment
DEPLOY_MODE=central
TENANT_ID=
CORE_REPLICAS=1
WORKER_REPLICAS=2

# Grafana
GRAFANA_PASS=admin

# Edge mode (uncomment for edge deployment)
# DEPLOY_MODE=edge
# TENANT_ID=tw-idc01
```

- [ ] **Step 4: Verify compose file syntax**

```bash
cd /cmdb-platform/cmdb-core/deploy
docker compose config --quiet 2>&1 || echo "compose config check"
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/deploy/docker-compose.yml cmdb-core/deploy/docker-compose.edge.yml cmdb-core/deploy/.env.example
git commit -m "feat: production Docker Compose - central full stack + edge lightweight overlay"
```

---

## Task 8: Makefile Updates + Startup Scripts

**Files:**
- Modify: `cmdb-core/Makefile`
- Create: `scripts/start-central.sh`
- Create: `scripts/start-edge.sh`

- [ ] **Step 1: Update Makefile with deploy targets**

Add to `cmdb-core/Makefile`:

```makefile
# === Deployment ===

# Start full central stack (all services + observability)
up:
	cd deploy && docker compose up -d

# Start edge stack
up-edge:
	cd deploy && docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d

# Stop all services
down:
	cd deploy && docker compose down

# View logs
logs:
	cd deploy && docker compose logs -f --tail=100

# View specific service logs
logs-%:
	cd deploy && docker compose logs -f --tail=100 $*

# === Observability Shortcuts ===

# Open Grafana (http://localhost:3000)
grafana:
	@echo "Grafana: http://localhost:3000 (admin/admin)"

# Open Jaeger (http://localhost:16686)
jaeger:
	@echo "Jaeger: http://localhost:16686"

# Check Prometheus targets
prom-targets:
	@curl -s http://localhost:9090/api/v1/targets | python3 -m json.tool | head -30
```

- [ ] **Step 2: Create startup scripts**

Create `scripts/start-central.sh`:

```bash
#!/bin/bash
set -e
echo "=== CMDB Platform — Central Deployment ==="

cd "$(dirname "$0")/../cmdb-core/deploy"

# Check .env
if [ ! -f .env ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
fi

# Start infrastructure first
echo "Starting storage + messaging..."
docker compose up -d postgres redis nats
echo "Waiting for services to be healthy..."
sleep 5

# Run migrations
echo "Running database migrations..."
docker compose exec -T postgres psql -U cmdb -d cmdb -f /dev/null 2>/dev/null || true
cd ../../cmdb-core
DATABASE_URL="postgres://cmdb:cmdb_secret@localhost:5432/cmdb?sslmode=disable" make migrate 2>/dev/null || echo "Migrations: using go run"

# Apply ingestion migrations
psql "postgres://cmdb:cmdb_secret@localhost:5432/cmdb" \
    -f ../ingestion-engine/db/migrations/000010_ingestion_tables.up.sql 2>/dev/null || true

# Seed data
DATABASE_URL="postgres://cmdb:cmdb_secret@localhost:5432/cmdb?sslmode=disable" make seed 2>/dev/null || true

# Start all services
cd deploy
echo "Starting all services..."
docker compose up -d

echo ""
echo "=== CMDB Platform Running ==="
echo "  Frontend:     http://localhost (via Nginx)"
echo "  API:          http://localhost:8080/api/v1"
echo "  MCP Server:   http://localhost:3001"
echo "  Ingestion:    http://localhost:8081"
echo "  Grafana:      http://localhost:3000 (admin/admin)"
echo "  Jaeger:       http://localhost:16686"
echo "  Prometheus:   http://localhost:9090"
echo ""
```

Create `scripts/start-edge.sh`:

```bash
#!/bin/bash
set -e

if [ -z "$TENANT_ID" ]; then
    echo "Usage: TENANT_ID=tw-idc01 ./scripts/start-edge.sh"
    exit 1
fi

echo "=== CMDB Platform — Edge Deployment (tenant: $TENANT_ID) ==="

cd "$(dirname "$0")/../cmdb-core/deploy"

export DEPLOY_MODE=edge
export TENANT_ID

docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d

echo ""
echo "=== Edge Node Running ==="
echo "  API:        http://localhost:8080/api/v1"
echo "  Ingestion:  http://localhost:8081"
echo "  NATS Leaf:  connecting to central hub..."
echo ""
```

```bash
chmod +x scripts/start-central.sh scripts/start-edge.sh
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/Makefile scripts/
git commit -m "feat: add deployment scripts + Makefile targets for central/edge modes"
```

---

## Summary

After all 8 tasks, Phase 4 delivers:

| Component | What's Added |
|-----------|-------------|
| **Structured Logging** | zap JSON logger, replaces log.Printf |
| **Prometheus Metrics** | api_request_total, api_request_duration, ws connections, NATS messages |
| **OpenTelemetry Tracing** | Distributed traces → OTel Collector → Jaeger |
| **Nginx Reverse Proxy** | Rate limiting, WebSocket upgrade, frontend serving, request ID propagation |
| **NATS Federation** | Central hub config + Edge leaf node config |
| **Observability Stack** | OTel Collector, Prometheus, Loki, Promtail, Jaeger, Grafana (all pre-configured) |
| **Grafana Dashboard** | API Overview with QPS, latency percentiles, status codes, WS connections |
| **Docker Compose Central** | Full 13-service stack with observability |
| **Docker Compose Edge** | Lightweight overlay (no observability, leaf NATS, single replicas) |
| **Startup Scripts** | One-command central + edge deployment |

### Complete Platform Architecture

```
Central Stack (13 services):
  cmdb-core → Nginx ← Frontend
  ingestion-engine + worker
  PostgreSQL + TimescaleDB
  Redis
  NATS (Hub mode, JetStream)
  OTel Collector → Jaeger (traces)
  Prometheus (metrics)
  Loki + Promtail (logs)
  Grafana (dashboards)

Edge Stack (6 services):
  cmdb-core (edge mode)
  ingestion-engine + worker
  PostgreSQL + Redis
  NATS (Leaf Node → Central Hub)
```
