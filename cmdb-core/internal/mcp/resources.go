package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerResources registers all 3 MCP resources on the server.
func (s *MCPServer) registerResources() {
	// 1. Static: asset type schema
	s.srv.AddResource(
		mcp.NewResource(
			"cmdb://schema/asset-types",
			"Asset Types Schema",
			mcp.WithResourceDescription("Available asset types, subtypes, statuses, and BIA levels"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleAssetTypesSchema,
	)

	// 2. Static: severity levels
	s.srv.AddResource(
		mcp.NewResource(
			"cmdb://schema/severity-levels",
			"Severity Levels",
			mcp.WithResourceDescription("Alert severity level definitions"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleSeverityLevels,
	)

	// 3. Dynamic: topology tree (root locations)
	s.srv.AddResource(
		mcp.NewResource(
			"cmdb://topology/tree",
			"Topology Tree",
			mcp.WithResourceDescription("Root location hierarchy for the default tenant"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleTopologyTree,
	)
}

// --- Resource handlers ---

func (s *MCPServer) handleAssetTypesSchema(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	schema := map[string]any{
		"types": []string{
			"server", "network", "storage", "security",
			"power", "cooling", "cable", "other",
		},
		"subtypes": map[string][]string{
			"server":  {"physical", "virtual", "container"},
			"network": {"switch", "router", "firewall", "load_balancer", "access_point"},
			"storage": {"san", "nas", "das", "object"},
		},
		"statuses": []string{
			"active", "maintenance", "decommissioned", "staged", "in_transit",
		},
		"bia_levels": []string{
			"critical", "high", "medium", "low",
		},
	}

	b, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "cmdb://schema/asset-types",
			MIMEType: "application/json",
			Text:     string(b),
		},
	}, nil
}

func (s *MCPServer) handleSeverityLevels(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	schema := map[string]any{
		"levels": []map[string]any{
			{
				"name":        "critical",
				"priority":    1,
				"description": "Service-impacting event requiring immediate response",
				"color":       "#dc2626",
			},
			{
				"name":        "warning",
				"priority":    2,
				"description": "Potential issue that may escalate if not addressed",
				"color":       "#f59e0b",
			},
			{
				"name":        "info",
				"priority":    3,
				"description": "Informational event for awareness and tracking",
				"color":       "#3b82f6",
			},
		},
	}

	b, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "cmdb://schema/severity-levels",
			MIMEType: "application/json",
			Text:     string(b),
		},
	}, nil
}

func (s *MCPServer) handleTopologyTree(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	tid, err := s.defaultTenantID(ctx)
	if err != nil {
		return nil, err
	}

	roots, err := s.queries.ListRootLocations(ctx, tid)
	if err != nil {
		return nil, err
	}

	b, err := json.MarshalIndent(roots, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "cmdb://topology/tree",
			MIMEType: "application/json",
			Text:     string(b),
		},
	}, nil
}
