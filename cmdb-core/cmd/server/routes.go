package main

// routes.go — beginning of the main.go split per Wave 1 Foundation.
//
// This file is intentionally thin right now. Future waves will migrate
// progressively more route registration here so main.go shrinks toward
// pure process lifecycle (flags, signal handling, shutdown).
//
// Today this file houses:
//   - infraMiddleware: the ordered middleware chain that runs on every
//     request (tracing → recovery → CORS → security headers → request
//     id → prometheus).
//   - registerPublicRoutes: /healthz, /readyz, /metrics — routes that
//     must be reachable without auth for probes and scrapers.
//
// It explicitly does NOT yet own the /api/v1 group or the auth wiring;
// those stay in main.go until Wave 11 because they require more careful
// dependency threading.

import (
	"github.com/cmdb-platform/cmdb-core/internal/api"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/gin-gonic/gin"
)

// infraMiddleware installs the per-request middleware every route must run
// through, in dependency order. Tracing comes first so spans wrap every
// subsequent middleware. Recovery is next so a panic in any later middleware
// (CORS, security, etc.) still produces a 500 instead of crashing the
// process. Prometheus middleware comes last so it measures the fully-wrapped
// request duration, not partial.
func infraMiddleware(router *gin.Engine) {
	router.Use(telemetry.TracingMiddleware("cmdb-core"))
	router.Use(
		middleware.Recovery(),
		middleware.CORS(),
		middleware.SecurityHeaders(),
		middleware.RequestID(),
	)
	router.Use(telemetry.PrometheusMiddleware())
}

// registerPublicRoutes mounts the three top-level endpoints that must be
// reachable without any /api/v1 machinery (health probes for K8s, scrape
// target for Prometheus). Keeping them separate from the v1 group prevents
// future middleware mistakes from blocking liveness/readiness checks.
func registerPublicRoutes(router *gin.Engine, health *api.HealthHandler) {
	router.GET("/healthz", health.Liveness)
	router.GET("/readyz", health.Readiness)
	router.GET("/metrics", telemetry.MetricsHandler())
}
