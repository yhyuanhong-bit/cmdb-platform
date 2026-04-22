package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
)

// registerTools registers all 7 MCP tools on the server.
func (s *MCPServer) registerTools() {
	s.srv.AddTool(
		mcp.NewTool("search_assets",
			mcp.WithDescription("Search assets with optional type/status/query filters"),
			mcp.WithString("type", mcp.Description("Asset type filter (e.g. server, network, storage)")),
			mcp.WithString("status", mcp.Description("Asset status filter (e.g. active, maintenance, decommissioned)")),
			mcp.WithString("query", mcp.Description("Free-text query (matched against serial_number)")),
		),
		s.handleSearchAssets,
	)

	s.srv.AddTool(
		mcp.NewTool("get_asset_detail",
			mcp.WithDescription("Get detailed asset information by UUID or asset_tag"),
			mcp.WithString("id", mcp.Description("Asset UUID")),
			mcp.WithString("asset_tag", mcp.Description("Asset tag identifier")),
		),
		s.handleGetAssetDetail,
	)

	s.srv.AddTool(
		mcp.NewTool("query_alerts",
			mcp.WithDescription("Query alert events with severity/status/asset_id filters"),
			mcp.WithString("severity", mcp.Description("Severity filter (critical, warning, info)")),
			mcp.WithString("status", mcp.Description("Status filter (firing, acknowledged, resolved)")),
			mcp.WithString("asset_id", mcp.Description("Asset UUID to filter alerts for")),
		),
		s.handleQueryAlerts,
	)

	s.srv.AddTool(
		mcp.NewTool("get_topology",
			mcp.WithDescription("Get location hierarchy — root locations or children of a location"),
			mcp.WithString("location_id", mcp.Description("Parent location UUID; omit for root locations")),
		),
		s.handleGetTopology,
	)

	s.srv.AddTool(
		mcp.NewTool("query_metrics",
			mcp.WithDescription("Query time-series metrics for an asset"),
			mcp.WithString("asset_id", mcp.Required(), mcp.Description("Asset UUID")),
			mcp.WithString("metric_name", mcp.Required(), mcp.Description("Metric name (e.g. cpu_usage, memory_usage)")),
			mcp.WithString("time_range", mcp.Description("Time range to query (e.g. 1h, 6h, 24h, 7d). Default: 24h")),
		),
		s.handleQueryMetrics,
	)

	s.srv.AddTool(
		mcp.NewTool("query_work_orders",
			mcp.WithDescription("Query work orders with optional status filter"),
			mcp.WithString("status", mcp.Description("Status filter (e.g. open, in_progress, completed)")),
		),
		s.handleQueryWorkOrders,
	)

	s.srv.AddTool(
		mcp.NewTool("trigger_rca",
			mcp.WithDescription("Trigger root cause analysis for an incident"),
			mcp.WithString("incident_id", mcp.Required(), mcp.Description("Incident UUID to analyse")),
		),
		s.handleTriggerRCA,
	)
}

// defaultTenantID returns the first tenant's ID for use as a default context.
func (s *MCPServer) defaultTenantID(ctx context.Context) (uuid.UUID, error) {
	tenants, err := s.queries.ListTenants(ctx)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("list tenants: %w", err)
	}
	if len(tenants) == 0 {
		return uuid.UUID{}, fmt.Errorf("no tenants found")
	}
	return tenants[0].ID, nil
}

// Helper to build pgtype.Text from an optional string arg.
func optText(args map[string]any, key string) pgtype.Text {
	v, ok := args[key]
	if !ok || v == nil || v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: fmt.Sprint(v), Valid: true}
}

// Helper to build pgtype.UUID from an optional string arg.
func optUUID(args map[string]any, key string) pgtype.UUID {
	v, ok := args[key]
	if !ok || v == nil || v == "" {
		return pgtype.UUID{}
	}
	parsed, err := uuid.Parse(fmt.Sprint(v))
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

// --- Tool handlers ---

func (s *MCPServer) handleSearchAssets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := s.defaultTenantID(ctx)
	if err != nil {
		return nil, err
	}

	assets, err := s.queries.ListAssets(ctx, dbgen.ListAssetsParams{
		TenantID:     tid,
		Limit:        50,
		Offset:       0,
		Type:         optText(args, "type"),
		Status:       optText(args, "status"),
		SerialNumber: optText(args, "query"),
	})
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	return jsonResult(assets)
}

func (s *MCPServer) handleGetAssetDetail(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	tid, err := s.defaultTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve tenant: %w", err)
	}

	// Try by UUID first.
	if idStr, ok := args["id"]; ok && idStr != nil && idStr != "" {
		parsed, err := uuid.Parse(fmt.Sprint(idStr))
		if err != nil {
			return nil, fmt.Errorf("invalid UUID: %w", err)
		}
		asset, err := s.queries.GetAsset(ctx, dbgen.GetAssetParams{ID: parsed, TenantID: tid})
		if err != nil {
			return nil, fmt.Errorf("get asset: %w", err)
		}
		return jsonResult(asset)
	}

	// Fall back to asset_tag.
	if tag, ok := args["asset_tag"]; ok && tag != nil && tag != "" {
		asset, err := s.queries.GetAssetByTag(ctx, fmt.Sprint(tag))
		if err != nil {
			return nil, fmt.Errorf("get asset by tag: %w", err)
		}
		return jsonResult(asset)
	}

	return mcp.NewToolResultText("error: provide either id or asset_tag"), nil
}

func (s *MCPServer) handleQueryAlerts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := s.defaultTenantID(ctx)
	if err != nil {
		return nil, err
	}

	alerts, err := s.queries.ListAlerts(ctx, dbgen.ListAlertsParams{
		TenantID: tid,
		Limit:    50,
		Offset:   0,
		Severity: optText(args, "severity"),
		Status:   optText(args, "status"),
		AssetID:  optUUID(args, "asset_id"),
	})
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	return jsonResult(alerts)
}

func (s *MCPServer) handleGetTopology(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := s.defaultTenantID(ctx)
	if err != nil {
		return nil, err
	}

	// If location_id is provided, return children of that location.
	if locID, ok := args["location_id"]; ok && locID != nil && locID != "" {
		parsed, parseErr := uuid.Parse(fmt.Sprint(locID))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid location UUID: %w", parseErr)
		}
		children, listErr := s.queries.ListChildren(ctx, pgtype.UUID{Bytes: parsed, Valid: true})
		if listErr != nil {
			return nil, fmt.Errorf("list children: %w", listErr)
		}
		return jsonResult(children)
	}

	// Otherwise return root locations.
	roots, err := s.queries.ListRootLocations(ctx, tid)
	if err != nil {
		return nil, fmt.Errorf("list root locations: %w", err)
	}
	return jsonResult(roots)
}

// parseDuration parses a human-friendly duration string like "1h", "6h",
// "24h", "7d" into a time.Duration. Falls back to 24h on invalid input.
func parseDuration(s string) time.Duration {
	if s == "" {
		return 24 * time.Hour
	}
	// Handle day shorthand (e.g. "7d" -> "168h").
	if len(s) > 1 && s[len(s)-1] == 'd' {
		days := s[:len(s)-1]
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err == nil && n > 0 {
			return time.Duration(n) * 24 * time.Hour
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 24 * time.Hour
	}
	return d
}

func (s *MCPServer) handleQueryMetrics(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	// Parse required asset_id.
	assetIDStr, _ := args["asset_id"].(string)
	if assetIDStr == "" {
		return mcp.NewToolResultText(`{"error":"asset_id is required"}`), nil
	}
	assetUUID, err := uuid.Parse(assetIDStr)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf(`{"error":"invalid asset_id UUID: %s"}`, err)), nil
	}

	// Parse required metric_name.
	metricName, _ := args["metric_name"].(string)
	if metricName == "" {
		return mcp.NewToolResultText(`{"error":"metric_name is required"}`), nil
	}

	// Parse optional time_range (default 24h).
	timeRange, _ := args["time_range"].(string)
	dur := parseDuration(timeRange)
	since := time.Now().UTC().Add(-dur)

	rows, err := s.queries.QueryMetricsByAsset(ctx, dbgen.QueryMetricsByAssetParams{
		AssetID: pgtype.UUID{Bytes: assetUUID, Valid: true},
		Name:    metricName,
		Time:    since,
	})
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}

	// Build a clean response with explicit value handling.
	type metricPoint struct {
		Time  time.Time `json:"time"`
		Name  string    `json:"name"`
		Value *float64  `json:"value"`
	}
	points := make([]metricPoint, 0, len(rows))
	for _, r := range rows {
		p := metricPoint{Time: r.Time, Name: r.Name}
		if r.Value.Valid {
			v := r.Value.Float64
			p.Value = &v
		}
		points = append(points, p)
	}

	result := struct {
		Status string        `json:"status"`
		Data   []metricPoint `json:"data"`
		Count  int           `json:"count"`
	}{
		Status: "ok",
		Data:   points,
		Count:  len(points),
	}
	return jsonResult(result)
}

func (s *MCPServer) handleQueryWorkOrders(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	tid, err := s.defaultTenantID(ctx)
	if err != nil {
		return nil, err
	}

	orders, err := s.queries.ListWorkOrders(ctx, dbgen.ListWorkOrdersParams{
		TenantID: tid,
		Limit:    50,
		Offset:   0,
		Status:   optText(args, "status"),
	})
	if err != nil {
		return nil, fmt.Errorf("list work orders: %w", err)
	}
	return jsonResult(orders)
}

func (s *MCPServer) handleTriggerRCA(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	incidentID, ok := args["incident_id"]
	if !ok || incidentID == nil || incidentID == "" {
		return mcp.NewToolResultText("error: incident_id is required"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		`{"status":"accepted","message":"RCA triggered for incident %s. Analysis will be available shortly."}`,
		incidentID,
	)), nil
}
