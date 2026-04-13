// Package mcp provides an MCP (Model Context Protocol) server for AI agent
// integration with CMDB data.
package mcp

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
)

// MCPServer wraps the mcp-go server and provides CMDB-specific tools and
// resources for AI agents.
type MCPServer struct {
	queries *dbgen.Queries
	srv     *server.MCPServer
}

// New creates a new MCPServer backed by the given database queries.
func New(q *dbgen.Queries) *MCPServer {
	s := &MCPServer{
		queries: q,
		srv:     server.NewMCPServer("cmdb-platform", "1.0.0"),
	}
	s.registerTools()
	s.registerResources()
	return s
}

// Server returns the underlying mcp-go server for transport binding.
func (s *MCPServer) Server() *server.MCPServer {
	return s.srv
}

// jsonResult marshals v to indented JSON and wraps it in a CallToolResult.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(b)), nil
}
