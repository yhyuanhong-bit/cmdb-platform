# Phase 3: Real Data Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 5 new database tables, 15 Go API endpoints, and connect 6 frontend pages to real data, eliminating all remaining hardcoded mock data.

**Architecture:** New Go handler files (phase3_*.go) with raw SQL on pgxpool, registered as custom routes in main.go. Frontend extends existing API clients and hooks. Force-directed layout for topology graph.

**Tech Stack:** Go/Gin, pgxpool raw SQL, React/TypeScript, TanStack React Query.

**Spec:** `docs/superpowers/specs/2026-04-07-phase3-real-data-integration-design.md`

---

## File Structure

### New Files (Go Backend)

| File | Responsibility |
|------|---------------|
| `cmdb-core/db/migrations/000018_phase3_tables.up.sql` | 5 new tables |
| `cmdb-core/db/migrations/000018_phase3_tables.down.sql` | Rollback |
| `cmdb-core/internal/api/phase3_inventory_endpoints.go` | Scan history + notes handlers (4) |
| `cmdb-core/internal/api/phase3_maintenance_endpoints.go` | Comment handlers (2) |
| `cmdb-core/internal/api/phase3_topology_endpoints.go` | Dependencies + graph handlers (4) |
| `cmdb-core/internal/api/phase3_rack_endpoints.go` | Network connection handlers (3) |
| `cmdb-core/internal/api/phase3_activity_endpoints.go` | Activity feed + audit detail handlers (2) |

### New Files (Frontend)

| File | Responsibility |
|------|---------------|
| `cmdb-demo/src/lib/api/activity.ts` | Activity feed API client |
| `cmdb-demo/src/hooks/useActivityFeed.ts` | Activity feed hook |
| `cmdb-demo/src/hooks/useForceLayout.ts` | Force-directed graph layout |

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-core/cmd/server/main.go` | Register 15 new routes |
| `cmdb-demo/src/lib/api/topology.ts` | Add 7 methods (dependencies, graph, network-connections) |
| `cmdb-demo/src/lib/api/inventory.ts` | Add 4 methods (scan-history, notes) |
| `cmdb-demo/src/lib/api/maintenance.ts` | Add 2 methods (comments) |
| `cmdb-demo/src/lib/api/audit.ts` | Add 1 method (getEventById) |
| `cmdb-demo/src/hooks/useTopology.ts` | Add 7 hooks |
| `cmdb-demo/src/hooks/useInventory.ts` | Add 4 hooks |
| `cmdb-demo/src/hooks/useMaintenance.ts` | Add 2 hooks |
| `cmdb-demo/src/hooks/useAudit.ts` | Add 1 hook (useAuditEventDetail) |
| `cmdb-demo/src/pages/InventoryItemDetail.tsx` | Replace SCAN_HISTORY + DISCREPANCY_NOTES |
| `cmdb-demo/src/pages/MaintenanceTaskView.tsx` | Wire comment submit + list |
| `cmdb-demo/src/pages/RackDetailUnified.tsx` | Replace networkConnections + recentActivity |
| `cmdb-demo/src/pages/RackManagement.tsx` | Replace recentEvents |
| `cmdb-demo/src/pages/AlertTopologyAnalysis.tsx` | Replace NODES/EDGES + force layout |
| `cmdb-demo/src/pages/AuditEventDetail.tsx` | Replace FALLBACK_EVENT/DIFF |

---

## Task 1: Database Migration

**Files:**
- Create: `cmdb-core/db/migrations/000018_phase3_tables.up.sql`
- Create: `cmdb-core/db/migrations/000018_phase3_tables.down.sql`

- [ ] **Step 1: Write the up migration**

Create `cmdb-core/db/migrations/000018_phase3_tables.up.sql` with the exact SQL from the spec (Section 2). The file contains 5 CREATE TABLE statements + indexes for: `inventory_scan_history`, `inventory_notes`, `work_order_comments`, `asset_dependencies`, `rack_network_connections`.

- [ ] **Step 2: Write the down migration**

Create `cmdb-core/db/migrations/000018_phase3_tables.down.sql`:
```sql
DROP TABLE IF EXISTS rack_network_connections;
DROP TABLE IF EXISTS asset_dependencies;
DROP TABLE IF EXISTS work_order_comments;
DROP TABLE IF EXISTS inventory_notes;
DROP TABLE IF EXISTS inventory_scan_history;
```

- [ ] **Step 3: Run the migration**

```bash
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -f cmdb-core/db/migrations/000018_phase3_tables.up.sql
```

- [ ] **Step 4: Verify all tables exist**

```bash
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "\d inventory_scan_history"
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "\d inventory_notes"
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "\d work_order_comments"
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "\d asset_dependencies"
psql "postgresql://cmdb:changeme@localhost:5432/cmdb" -c "\d rack_network_connections"
```

- [ ] **Step 5: Commit**

```bash
git add cmdb-core/db/migrations/000018_phase3_tables.up.sql cmdb-core/db/migrations/000018_phase3_tables.down.sql
git commit -m "feat: add Phase 3 tables (scan_history, notes, comments, deps, net_conn)"
```

---

## Task 2: Inventory Scan History + Notes Endpoints (Go)

**Files:**
- Create: `cmdb-core/internal/api/phase3_inventory_endpoints.go`

- [ ] **Step 1: Create the endpoint file**

Create `cmdb-core/internal/api/phase3_inventory_endpoints.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetItemScanHistory handles GET /inventory/tasks/:id/items/:itemId/scan-history
func (s *APIServer) GetItemScanHistory(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT ish.id, ish.scanned_at, u.display_name, ish.method, ish.result, ish.note
		FROM inventory_scan_history ish
		LEFT JOIN users u ON ish.scanned_by = u.id
		WHERE ish.item_id = $1
		ORDER BY ish.scanned_at DESC
	`, itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var results []gin.H
	for rows.Next() {
		var id uuid.UUID
		var scannedAt time.Time
		var operator, method, result *string
		var note *string
		if err := rows.Scan(&id, &scannedAt, &operator, &method, &result, &note); err != nil {
			continue
		}
		results = append(results, gin.H{
			"id":        id.String(),
			"timestamp": scannedAt,
			"operator":  operator,
			"method":    method,
			"result":    result,
			"note":      note,
		})
	}
	if results == nil {
		results = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"scan_history": results})
}

// CreateItemScanRecord handles POST /inventory/tasks/:id/items/:itemId/scan-history
func (s *APIServer) CreateItemScanRecord(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}
	userID := userIDFromContext(c)

	var req struct {
		Method string  `json:"method"`
		Result string  `json:"result"`
		Note   *string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	id := uuid.New()
	_, err = s.pool.Exec(c.Request.Context(), `
		INSERT INTO inventory_scan_history (id, item_id, scanned_by, method, result, note)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, itemID, userID, req.Method, req.Result, req.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

// GetItemNotes handles GET /inventory/tasks/:id/items/:itemId/notes
func (s *APIServer) GetItemNotes(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT n.id, n.created_at, u.display_name, n.severity, n.text
		FROM inventory_notes n
		LEFT JOIN users u ON n.author_id = u.id
		WHERE n.item_id = $1
		ORDER BY n.created_at DESC
	`, itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var results []gin.H
	for rows.Next() {
		var id uuid.UUID
		var createdAt time.Time
		var author, severity, text *string
		if err := rows.Scan(&id, &createdAt, &author, &severity, &text); err != nil {
			continue
		}
		results = append(results, gin.H{
			"id":        id.String(),
			"timestamp": createdAt,
			"author":    author,
			"severity":  severity,
			"text":      text,
		})
	}
	if results == nil {
		results = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"notes": results})
}

// CreateItemNote handles POST /inventory/tasks/:id/items/:itemId/notes
func (s *APIServer) CreateItemNote(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}
	userID := userIDFromContext(c)

	var req struct {
		Severity string `json:"severity"`
		Text     string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Severity == "" {
		req.Severity = "info"
	}

	id := uuid.New()
	_, err = s.pool.Exec(c.Request.Context(), `
		INSERT INTO inventory_notes (id, item_id, author_id, severity, text)
		VALUES ($1, $2, $3, $4, $5)
	`, id, itemID, userID, req.Severity, req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}
```

- [ ] **Step 2: Build to verify compilation**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

Note: This will fail because routes aren't registered yet. That's OK — we just need the file to compile. If there are syntax errors, fix them now.

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/internal/api/phase3_inventory_endpoints.go
git commit -m "feat: add inventory scan-history and notes Go endpoints"
```

---

## Task 3: Maintenance Comments Endpoints (Go)

**Files:**
- Create: `cmdb-core/internal/api/phase3_maintenance_endpoints.go`

- [ ] **Step 1: Create the endpoint file**

Create `cmdb-core/internal/api/phase3_maintenance_endpoints.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetWorkOrderComments handles GET /maintenance/orders/:id/comments
func (s *APIServer) GetWorkOrderComments(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT wc.id, u.display_name, wc.text, wc.created_at
		FROM work_order_comments wc
		LEFT JOIN users u ON wc.author_id = u.id
		WHERE wc.order_id = $1
		ORDER BY wc.created_at ASC
	`, orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var comments []gin.H
	for rows.Next() {
		var id uuid.UUID
		var authorName *string
		var text string
		var createdAt time.Time
		if err := rows.Scan(&id, &authorName, &text, &createdAt); err != nil {
			continue
		}
		comments = append(comments, gin.H{
			"id":          id.String(),
			"author_name": authorName,
			"text":        text,
			"created_at":  createdAt,
		})
	}
	if comments == nil {
		comments = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

// CreateWorkOrderComment handles POST /maintenance/orders/:id/comments
func (s *APIServer) CreateWorkOrderComment(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	userID := userIDFromContext(c)

	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	id := uuid.New()
	_, err = s.pool.Exec(c.Request.Context(), `
		INSERT INTO work_order_comments (id, order_id, author_id, text)
		VALUES ($1, $2, $3, $4)
	`, id, orderID, userID, req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-core/internal/api/phase3_maintenance_endpoints.go
git commit -m "feat: add work order comments Go endpoints"
```

---

## Task 4: Topology Dependencies + Graph Endpoints (Go)

**Files:**
- Create: `cmdb-core/internal/api/phase3_topology_endpoints.go`

- [ ] **Step 1: Create the endpoint file**

Create `cmdb-core/internal/api/phase3_topology_endpoints.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetAssetDependencies handles GET /topology/dependencies?asset_id=
func (s *APIServer) GetAssetDependencies(c *gin.Context) {
	assetIDStr := c.Query("asset_id")
	if assetIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset_id query param required"})
		return
	}
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset_id"})
		return
	}
	tenantID := tenantIDFromContext(c)

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT ad.id, ad.source_asset_id, sa.name, ad.target_asset_id, ta.name,
		       ad.dependency_type, ad.description
		FROM asset_dependencies ad
		JOIN assets sa ON ad.source_asset_id = sa.id
		JOIN assets ta ON ad.target_asset_id = ta.id
		WHERE ad.tenant_id = $1 AND (ad.source_asset_id = $2 OR ad.target_asset_id = $2)
		ORDER BY ad.created_at DESC
	`, tenantID, assetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var deps []gin.H
	for rows.Next() {
		var id, srcID, tgtID uuid.UUID
		var srcName, tgtName, depType string
		var desc *string
		if err := rows.Scan(&id, &srcID, &srcName, &tgtID, &tgtName, &depType, &desc); err != nil {
			continue
		}
		deps = append(deps, gin.H{
			"id":                id.String(),
			"source_asset_id":   srcID.String(),
			"source_asset_name": srcName,
			"target_asset_id":   tgtID.String(),
			"target_asset_name": tgtName,
			"dependency_type":   depType,
			"description":       desc,
		})
	}
	if deps == nil {
		deps = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"dependencies": deps})
}

// CreateAssetDependency handles POST /topology/dependencies
func (s *APIServer) CreateAssetDependency(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	var req struct {
		SourceAssetID  string  `json:"source_asset_id"`
		TargetAssetID  string  `json:"target_asset_id"`
		DependencyType string  `json:"dependency_type"`
		Description    *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	srcID, _ := uuid.Parse(req.SourceAssetID)
	tgtID, _ := uuid.Parse(req.TargetAssetID)
	if srcID == uuid.Nil || tgtID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source and target asset IDs required"})
		return
	}
	if req.DependencyType == "" {
		req.DependencyType = "depends_on"
	}

	id := uuid.New()
	_, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO asset_dependencies (id, tenant_id, source_asset_id, target_asset_id, dependency_type, description)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, tenantID, srcID, tgtID, req.DependencyType, req.Description)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "dependency already exists or constraint violated"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

// DeleteAssetDependency handles DELETE /topology/dependencies/:id
func (s *APIServer) DeleteAssetDependency(c *gin.Context) {
	depID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	tag, err := s.pool.Exec(c.Request.Context(), `DELETE FROM asset_dependencies WHERE id = $1`, depID)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetTopologyGraph handles GET /topology/graph?location_id=
func (s *APIServer) GetTopologyGraph(c *gin.Context) {
	locIDStr := c.Query("location_id")
	if locIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "location_id required"})
		return
	}
	locID, err := uuid.Parse(locIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid location_id"})
		return
	}
	tenantID := tenantIDFromContext(c)

	// 1. Get assets in this location
	assetRows, err := s.pool.Query(c.Request.Context(), `
		SELECT a.id, a.name, a.type, a.sub_type, a.status, a.bia_level,
		       a.ip_address, a.model, r.name AS rack_name, a.tags,
		       EXISTS(SELECT 1 FROM alert_events ae WHERE ae.asset_id = a.id AND ae.status = 'firing') AS has_active_alert
		FROM assets a
		LEFT JOIN racks r ON a.rack_id = r.id
		WHERE a.tenant_id = $1 AND a.location_id = $2
		LIMIT 50
	`, tenantID, locID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query assets failed"})
		return
	}
	defer assetRows.Close()

	var nodes []gin.H
	assetIDs := []uuid.UUID{}
	alertMap := map[string]bool{}
	for assetRows.Next() {
		var id uuid.UUID
		var name, assetType, status, biaLevel string
		var subType, ipAddr, model, rackName *string
		var tags []string
		var hasAlert bool
		if err := assetRows.Scan(&id, &name, &assetType, &subType, &status, &biaLevel,
			&ipAddr, &model, &rackName, &tags, &hasAlert); err != nil {
			continue
		}
		assetIDs = append(assetIDs, id)
		alertMap[id.String()] = hasAlert
		node := gin.H{
			"id": id.String(), "name": name, "type": assetType,
			"sub_type": subType, "status": status, "bia_level": biaLevel,
			"ip_address": ipAddr, "model": model, "rack_name": rackName,
			"tags": tags, "has_active_alert": hasAlert, "metrics": nil,
		}
		nodes = append(nodes, node)
	}

	// 2. Get dependencies between these assets
	var edges []gin.H
	if len(assetIDs) > 0 {
		edgeRows, err := s.pool.Query(c.Request.Context(), `
			SELECT ad.id, ad.source_asset_id, ad.target_asset_id, ad.dependency_type, ad.description
			FROM asset_dependencies ad
			WHERE ad.tenant_id = $1
			  AND (ad.source_asset_id = ANY($2) OR ad.target_asset_id = ANY($2))
		`, tenantID, assetIDs)
		if err == nil {
			defer edgeRows.Close()
			for edgeRows.Next() {
				var eid, srcID, tgtID uuid.UUID
				var depType string
				var desc *string
				if err := edgeRows.Scan(&eid, &srcID, &tgtID, &depType, &desc); err != nil {
					continue
				}
				isFault := alertMap[srcID.String()] || alertMap[tgtID.String()]
				edges = append(edges, gin.H{
					"id": eid.String(), "from": srcID.String(), "to": tgtID.String(),
					"type": depType, "description": desc, "isFaultPath": isFault,
				})
			}
		}
	}
	if nodes == nil {
		nodes = []gin.H{}
	}
	if edges == nil {
		edges = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes, "edges": edges})
}
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-core/internal/api/phase3_topology_endpoints.go
git commit -m "feat: add topology dependencies and graph Go endpoints"
```

---

## Task 5: Rack Network Connections Endpoints (Go)

**Files:**
- Create: `cmdb-core/internal/api/phase3_rack_endpoints.go`

- [ ] **Step 1: Create the endpoint file**

Create `cmdb-core/internal/api/phase3_rack_endpoints.go`:

```go
package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetRackNetworkConnections handles GET /racks/:id/network-connections
func (s *APIServer) GetRackNetworkConnections(c *gin.Context) {
	rackID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rack id"})
		return
	}

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT rnc.id, rnc.source_port, rnc.connected_asset_id,
		       COALESCE(a.name, rnc.external_device) AS device,
		       rnc.external_device, rnc.speed, rnc.status, rnc.vlans, rnc.connection_type
		FROM rack_network_connections rnc
		LEFT JOIN assets a ON rnc.connected_asset_id = a.id
		WHERE rnc.rack_id = $1
		ORDER BY rnc.source_port
	`, rackID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var conns []gin.H
	for rows.Next() {
		var id uuid.UUID
		var port, device string
		var connAssetID *uuid.UUID
		var extDevice, speed, status, connType *string
		var vlans []int32
		if err := rows.Scan(&id, &port, &connAssetID, &device, &extDevice, &speed, &status, &vlans, &connType); err != nil {
			continue
		}
		// Format vlans array as comma-separated string
		vlanStrs := make([]string, len(vlans))
		for i, v := range vlans {
			vlanStrs[i] = fmt.Sprintf("%d", v)
		}
		vlanStr := strings.Join(vlanStrs, ",")

		conn := gin.H{
			"id": id.String(), "port": port, "device": device,
			"external_device": extDevice, "speed": speed,
			"status": status, "vlan": vlanStr, "connection_type": connType,
		}
		if connAssetID != nil {
			conn["connected_asset_id"] = connAssetID.String()
		}
		conns = append(conns, conn)
	}
	if conns == nil {
		conns = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"connections": conns})
}

// CreateRackNetworkConnection handles POST /racks/:id/network-connections
func (s *APIServer) CreateRackNetworkConnection(c *gin.Context) {
	rackID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rack id"})
		return
	}
	tenantID := tenantIDFromContext(c)

	var req struct {
		SourcePort       string    `json:"source_port"`
		ConnectedAssetID *string   `json:"connected_asset_id"`
		ExternalDevice   *string   `json:"external_device"`
		Speed            *string   `json:"speed"`
		Status           *string   `json:"status"`
		Vlans            []int32   `json:"vlans"`
		ConnectionType   *string   `json:"connection_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SourcePort == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_port required"})
		return
	}
	// Validate XOR constraint
	if req.ConnectedAssetID != nil && req.ExternalDevice != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide connected_asset_id OR external_device, not both"})
		return
	}

	var connAssetID *uuid.UUID
	if req.ConnectedAssetID != nil {
		parsed, _ := uuid.Parse(*req.ConnectedAssetID)
		connAssetID = &parsed
	}

	id := uuid.New()
	_, err = s.pool.Exec(c.Request.Context(), `
		INSERT INTO rack_network_connections
			(id, tenant_id, rack_id, source_port, connected_asset_id, external_device, speed, status, vlans, connection_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, id, tenantID, rackID, req.SourcePort, connAssetID, req.ExternalDevice,
		req.Speed, req.Status, req.Vlans, req.ConnectionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id.String()})
}

// DeleteRackNetworkConnection handles DELETE /racks/:id/network-connections/:connectionId
func (s *APIServer) DeleteRackNetworkConnection(c *gin.Context) {
	connID, err := uuid.Parse(c.Param("connectionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connectionId"})
		return
	}
	tag, err := s.pool.Exec(c.Request.Context(), `DELETE FROM rack_network_connections WHERE id = $1`, connID)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-core/internal/api/phase3_rack_endpoints.go
git commit -m "feat: add rack network connections Go endpoints"
```

---

## Task 6: Activity Feed + Audit Detail Endpoints (Go)

**Files:**
- Create: `cmdb-core/internal/api/phase3_activity_endpoints.go`

- [ ] **Step 1: Create the endpoint file**

Create `cmdb-core/internal/api/phase3_activity_endpoints.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetActivityFeed handles GET /activity-feed?target_type=rack&target_id=uuid
func (s *APIServer) GetActivityFeed(c *gin.Context) {
	targetType := c.Query("target_type")
	targetIDStr := c.Query("target_id")
	if targetType == "" || targetIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_type and target_id required"})
		return
	}
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_id"})
		return
	}
	tenantID := tenantIDFromContext(c)

	rows, err := s.pool.Query(c.Request.Context(), `
		(SELECT 'audit' AS event_type, ae.action,
		        ae.module || '.' || ae.action AS description,
		        ae.created_at AS timestamp, 'info' AS severity,
		        u.display_name AS operator
		 FROM audit_events ae
		 LEFT JOIN users u ON ae.operator_id = u.id
		 WHERE ae.target_type = $1 AND ae.target_id = $2 AND ae.tenant_id = $3)
		UNION ALL
		(SELECT 'alert', 'alert.' || al.status,
		        al.message, al.fired_at, al.severity, NULL
		 FROM alert_events al
		 WHERE al.tenant_id = $3
		   AND (
		     ($1 = 'rack' AND al.asset_id IN (SELECT rs.asset_id FROM rack_slots rs WHERE rs.rack_id = $2))
		     OR ($1 = 'asset' AND al.asset_id = $2)
		     OR ($1 = 'location' AND al.asset_id IN (
		       SELECT rs.asset_id FROM rack_slots rs JOIN racks r ON rs.rack_id = r.id WHERE r.location_id = $2))
		   ))
		UNION ALL
		(SELECT 'maintenance', wol.action,
		        COALESCE(wol.comment, wol.from_status || ' -> ' || wol.to_status),
		        wol.created_at, 'info', NULL
		 FROM work_order_logs wol
		 JOIN work_orders wo ON wol.order_id = wo.id
		 WHERE wo.tenant_id = $3
		   AND (
		     ($1 = 'rack' AND wo.asset_id IN (SELECT rs.asset_id FROM rack_slots rs WHERE rs.rack_id = $2))
		     OR ($1 = 'asset' AND wo.asset_id = $2)
		     OR ($1 = 'location' AND wo.asset_id IN (
		       SELECT rs.asset_id FROM rack_slots rs JOIN racks r ON rs.rack_id = r.id WHERE r.location_id = $2))
		   ))
		ORDER BY timestamp DESC LIMIT 20
	`, targetType, targetID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var events []gin.H
	for rows.Next() {
		var eventType, action, severity string
		var description, operator *string
		var ts time.Time
		if err := rows.Scan(&eventType, &action, &description, &ts, &severity, &operator); err != nil {
			continue
		}
		events = append(events, gin.H{
			"event_type":  eventType,
			"action":      action,
			"description": description,
			"timestamp":   ts,
			"severity":    severity,
			"operator":    operator,
		})
	}
	if events == nil {
		events = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// GetAuditEventDetail handles GET /audit/events/:id
func (s *APIServer) GetAuditEventDetail(c *gin.Context) {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id"})
		return
	}

	var id uuid.UUID
	var action, module, targetType, source string
	var targetID, operatorID pgtype.UUID
	var diff []byte
	var createdAt time.Time
	var operatorName, operatorEmail *string

	err = s.pool.QueryRow(c.Request.Context(), `
		SELECT ae.id, ae.action, ae.module, ae.target_type, ae.target_id,
		       ae.operator_id, ae.diff, ae.source, ae.created_at,
		       u.display_name, u.email
		FROM audit_events ae
		LEFT JOIN users u ON ae.operator_id = u.id
		WHERE ae.id = $1
	`, eventID).Scan(&id, &action, &module, &targetType, &targetID,
		&operatorID, &diff, &source, &createdAt,
		&operatorName, &operatorEmail)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}

	event := gin.H{
		"id": id.String(), "action": action, "module": module,
		"target_type": targetType, "source": source,
		"created_at": createdAt, "operator_name": operatorName,
		"operator_email": operatorEmail,
	}
	if targetID.Valid {
		event["target_id"] = uuid.UUID(targetID.Bytes).String()
	}
	if operatorID.Valid {
		event["operator_id"] = uuid.UUID(operatorID.Bytes).String()
	}
	if diff != nil {
		event["diff"] = string(diff)
	}
	c.JSON(http.StatusOK, gin.H{"event": event})
}
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-core/internal/api/phase3_activity_endpoints.go
git commit -m "feat: add activity feed and audit event detail Go endpoints"
```

---

## Task 7: Register All 15 Routes + Build

**Files:**
- Modify: `cmdb-core/cmd/server/main.go`

- [ ] **Step 1: Add route registrations**

In `cmdb-core/cmd/server/main.go`, after the existing Phase 2 custom routes (after line with `v1.GET("/inventory/tasks/:id/discrepancies"...)`), add:

```go
	// Phase 3 routes
	v1.GET("/inventory/tasks/:id/items/:itemId/scan-history", apiServer.GetItemScanHistory)
	v1.POST("/inventory/tasks/:id/items/:itemId/scan-history", apiServer.CreateItemScanRecord)
	v1.GET("/inventory/tasks/:id/items/:itemId/notes", apiServer.GetItemNotes)
	v1.POST("/inventory/tasks/:id/items/:itemId/notes", apiServer.CreateItemNote)
	v1.GET("/maintenance/orders/:id/comments", apiServer.GetWorkOrderComments)
	v1.POST("/maintenance/orders/:id/comments", apiServer.CreateWorkOrderComment)
	v1.GET("/topology/dependencies", apiServer.GetAssetDependencies)
	v1.POST("/topology/dependencies", apiServer.CreateAssetDependency)
	v1.DELETE("/topology/dependencies/:id", apiServer.DeleteAssetDependency)
	v1.GET("/topology/graph", apiServer.GetTopologyGraph)
	v1.GET("/racks/:id/network-connections", apiServer.GetRackNetworkConnections)
	v1.POST("/racks/:id/network-connections", apiServer.CreateRackNetworkConnection)
	v1.DELETE("/racks/:id/network-connections/:connectionId", apiServer.DeleteRackNetworkConnection)
	v1.GET("/activity-feed", apiServer.GetActivityFeed)
	v1.GET("/audit/events/:id", apiServer.GetAuditEventDetail)
```

- [ ] **Step 2: Build and verify**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

Expected: Build succeeds with no errors.

- [ ] **Step 3: Commit**

```bash
git add cmdb-core/cmd/server/main.go
git commit -m "feat: register 15 Phase 3 routes in main.go"
```

---

## Task 8: Frontend API Clients + Hooks

**Files:**
- Create: `cmdb-demo/src/lib/api/activity.ts`
- Create: `cmdb-demo/src/hooks/useActivityFeed.ts`
- Create: `cmdb-demo/src/hooks/useForceLayout.ts`
- Modify: `cmdb-demo/src/lib/api/topology.ts`
- Modify: `cmdb-demo/src/lib/api/inventory.ts`
- Modify: `cmdb-demo/src/lib/api/maintenance.ts`
- Modify: `cmdb-demo/src/lib/api/audit.ts`
- Modify: `cmdb-demo/src/hooks/useTopology.ts`
- Modify: `cmdb-demo/src/hooks/useInventory.ts`
- Modify: `cmdb-demo/src/hooks/useMaintenance.ts`
- Modify: `cmdb-demo/src/hooks/useAudit.ts`

This is a large task but all changes are mechanical — adding API methods and React Query hooks following existing patterns.

- [ ] **Step 1: Create activity API client**

Create `cmdb-demo/src/lib/api/activity.ts`:
```ts
import { apiClient } from './client'

export const activityApi = {
  getFeed: (params: Record<string, string>) =>
    apiClient.get('/activity-feed', params),
}
```

- [ ] **Step 2: Create useActivityFeed hook**

Create `cmdb-demo/src/hooks/useActivityFeed.ts`:
```ts
import { useQuery } from '@tanstack/react-query'
import { activityApi } from '../lib/api/activity'

export function useActivityFeed(targetType: string, targetId: string) {
  return useQuery({
    queryKey: ['activityFeed', targetType, targetId],
    queryFn: () => activityApi.getFeed({ target_type: targetType, target_id: targetId }),
    enabled: !!targetType && !!targetId,
  })
}
```

- [ ] **Step 3: Create useForceLayout hook**

Create `cmdb-demo/src/hooks/useForceLayout.ts` with the exact code from the spec (Section 4.4). Copy the full `useForceLayout` function with `useMemo`, force simulation, and boundary clamping. Add `import { useMemo } from 'react'` at the top.

- [ ] **Step 4: Extend topology.ts API client**

Read `cmdb-demo/src/lib/api/topology.ts`, then append these methods to the existing `topologyApi` object:
```ts
  // Dependencies
  listDependencies: (params: Record<string, string>) =>
    apiClient.get('/topology/dependencies', params),
  createDependency: (data: any) =>
    apiClient.post('/topology/dependencies', data),
  deleteDependency: (id: string) =>
    apiClient.del(`/topology/dependencies/${id}`),
  getTopologyGraph: (params: Record<string, string>) =>
    apiClient.get('/topology/graph', params),
  // Network connections
  listNetworkConnections: (rackId: string) =>
    apiClient.get(`/racks/${rackId}/network-connections`),
  createNetworkConnection: (rackId: string, data: any) =>
    apiClient.post(`/racks/${rackId}/network-connections`, data),
  deleteNetworkConnection: (rackId: string, connId: string) =>
    apiClient.del(`/racks/${rackId}/network-connections/${connId}`),
```

- [ ] **Step 5: Extend inventory API + hooks**

Read the existing inventory API file (likely `src/lib/api/inventory.ts`). Add:
```ts
  listScanHistory: (taskId: string, itemId: string) =>
    apiClient.get(`/inventory/tasks/${taskId}/items/${itemId}/scan-history`),
  createScanRecord: (taskId: string, itemId: string, data: any) =>
    apiClient.post(`/inventory/tasks/${taskId}/items/${itemId}/scan-history`, data),
  listNotes: (taskId: string, itemId: string) =>
    apiClient.get(`/inventory/tasks/${taskId}/items/${itemId}/notes`),
  createNote: (taskId: string, itemId: string, data: any) =>
    apiClient.post(`/inventory/tasks/${taskId}/items/${itemId}/notes`, data),
```

Read `src/hooks/useInventory.ts` and add 4 hooks:
```ts
export function useItemScanHistory(taskId: string, itemId: string) {
  return useQuery({
    queryKey: ['itemScanHistory', taskId, itemId],
    queryFn: () => inventoryApi.listScanHistory(taskId, itemId),
    enabled: !!taskId && !!itemId,
  })
}

export function useItemNotes(taskId: string, itemId: string) {
  return useQuery({
    queryKey: ['itemNotes', taskId, itemId],
    queryFn: () => inventoryApi.listNotes(taskId, itemId),
    enabled: !!taskId && !!itemId,
  })
}

export function useCreateItemScanRecord() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, itemId, data }: { taskId: string; itemId: string; data: any }) =>
      inventoryApi.createScanRecord(taskId, itemId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['itemScanHistory'] }),
  })
}

export function useCreateItemNote() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, itemId, data }: { taskId: string; itemId: string; data: any }) =>
      inventoryApi.createNote(taskId, itemId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['itemNotes'] }),
  })
}
```

- [ ] **Step 6: Extend maintenance API + hooks**

Read maintenance API file and add:
```ts
  listComments: (orderId: string) =>
    apiClient.get(`/maintenance/orders/${orderId}/comments`),
  createComment: (orderId: string, data: any) =>
    apiClient.post(`/maintenance/orders/${orderId}/comments`, data),
```

Read `src/hooks/useMaintenance.ts` and add:
```ts
export function useWorkOrderComments(orderId: string) {
  return useQuery({
    queryKey: ['workOrderComments', orderId],
    queryFn: () => maintenanceApi.listComments(orderId),
    enabled: !!orderId,
  })
}

export function useCreateWorkOrderComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ orderId, data }: { orderId: string; data: { text: string } }) =>
      maintenanceApi.createComment(orderId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['workOrderComments'] }),
  })
}
```

- [ ] **Step 7: Extend audit API + hooks**

Read audit API file and add:
```ts
  getEventById: (id: string) => apiClient.get(`/audit/events/${id}`),
```

Read `src/hooks/useAudit.ts` and add:
```ts
export function useAuditEventDetail(eventId: string) {
  return useQuery({
    queryKey: ['auditEvent', eventId],
    queryFn: () => auditApi.getEventById(eventId),
    enabled: !!eventId,
  })
}
```

- [ ] **Step 8: Extend useTopology.ts hooks**

Read `src/hooks/useTopology.ts` and add 7 hooks for: `useTopologyGraph`, `useAssetDependencies`, `useCreateDependency`, `useDeleteDependency`, `useRackNetworkConnections`, `useCreateNetworkConnection`, `useDeleteNetworkConnection`. Follow the exact pattern from existing hooks in the file.

- [ ] **Step 9: Commit**

```bash
git add cmdb-demo/src/lib/api/activity.ts cmdb-demo/src/hooks/useActivityFeed.ts \
       cmdb-demo/src/hooks/useForceLayout.ts cmdb-demo/src/lib/api/topology.ts \
       cmdb-demo/src/lib/api/inventory.ts cmdb-demo/src/lib/api/maintenance.ts \
       cmdb-demo/src/lib/api/audit.ts cmdb-demo/src/hooks/useTopology.ts \
       cmdb-demo/src/hooks/useInventory.ts cmdb-demo/src/hooks/useMaintenance.ts \
       cmdb-demo/src/hooks/useAudit.ts
git commit -m "feat: add Phase 3 frontend API clients, hooks, and force layout"
```

---

## Task 9: Connect InventoryItemDetail Page

**Files:**
- Modify: `cmdb-demo/src/pages/InventoryItemDetail.tsx`

- [ ] **Step 1: Read the current file and identify hardcoded data**

Read the full file. Find:
- `SCAN_HISTORY` constant (array of hardcoded scan records)
- `DISCREPANCY_NOTES` constant (array of hardcoded notes)
- "Add note" submit handler (currently `alert('Note saved')`)

- [ ] **Step 2: Replace hardcoded data with API hooks**

1. Import: `useItemScanHistory`, `useItemNotes`, `useCreateItemNote` from `../hooks/useInventory`
2. Get `taskId` and `itemId` from URL params or the task/item data
3. Replace `SCAN_HISTORY` with: `const { data: scanData } = useItemScanHistory(taskId, itemId)` → `const scanHistory = (scanData as any)?.scan_history ?? []`
4. Replace `DISCREPANCY_NOTES` with: `const { data: notesData } = useItemNotes(taskId, itemId)` → `const notes = (notesData as any)?.notes ?? []`
5. Wire "Add note" submit to: `createNote.mutate({ taskId, itemId, data: { severity, text } })`
6. Delete the hardcoded constant arrays
7. If API returns empty, show "No scan history" / "No notes" empty states

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/InventoryItemDetail.tsx
git commit -m "feat: connect InventoryItemDetail to scan-history and notes APIs"
```

---

## Task 10: Connect MaintenanceTaskView Page

**Files:**
- Modify: `cmdb-demo/src/pages/MaintenanceTaskView.tsx`

- [ ] **Step 1: Read the current file**

Find the comment textarea and submit button (currently `alert('Comment: Coming Soon')`).

- [ ] **Step 2: Wire comment submission and list**

1. Import: `useWorkOrderComments`, `useCreateWorkOrderComment` from `../hooks/useMaintenance`
2. Add: `const { data: commentsData } = useWorkOrderComments(orderId)`
3. Add: `const createComment = useCreateWorkOrderComment()`
4. Replace submit handler: `createComment.mutate({ orderId, data: { text: comment } })`, clear textarea on success
5. Add comment list section below textarea:
```tsx
{((commentsData as any)?.comments ?? []).map((c: any) => (
  <div key={c.id} className="border-t border-surface-container-high py-3">
    <div className="flex justify-between text-xs text-on-surface-variant">
      <span>{c.author_name}</span>
      <span>{new Date(c.created_at).toLocaleString()}</span>
    </div>
    <p className="text-sm text-on-surface mt-1">{c.text}</p>
  </div>
))}
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/MaintenanceTaskView.tsx
git commit -m "feat: connect MaintenanceTaskView comment submission to API"
```

---

## Task 11: Connect RackDetailUnified + RackManagement Pages

**Files:**
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`
- Modify: `cmdb-demo/src/pages/RackManagement.tsx`

- [ ] **Step 1: RackDetailUnified — replace networkConnections**

1. Read current file, find `networkConnections` hardcoded array
2. Import `useRackNetworkConnections` from `../hooks/useTopology`
3. Replace hardcoded array: `const { data: netData } = useRackNetworkConnections(rackId)` → `const networkConnections = (netData as any)?.connections ?? []`
4. The existing table renders `conn.port`, `conn.device`, `conn.speed`, `conn.status`, `conn.vlan` — these match the API response field names
5. Delete the hardcoded constant

- [ ] **Step 2: RackDetailUnified — replace recentActivity**

1. Import `useActivityFeed` from `../hooks/useActivityFeed`
2. Replace hardcoded array: `const { data: activityData } = useActivityFeed('rack', rackId)` → `const recentActivity = (activityData as any)?.events ?? []`
3. Map API fields to display: `action` → `event.description`, `time` → format `event.timestamp`, derive `icon` from `event.event_type` (audit→history, alert→warning, maintenance→build)
4. Delete the hardcoded constant

- [ ] **Step 3: RackManagement — replace recentEvents**

1. Read current file, find `recentEvents` hardcoded array
2. Import `useActivityFeed` from `../hooks/useActivityFeed`
3. Get current location ID from context or rack data
4. Replace: `const { data: feedData } = useActivityFeed('location', locationId)` → `const recentEvents = (feedData as any)?.events ?? []`
5. Map API fields similarly to Step 2
6. Delete the hardcoded constant

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/RackDetailUnified.tsx cmdb-demo/src/pages/RackManagement.tsx
git commit -m "feat: connect RackDetail and RackManagement to network connections and activity feed APIs"
```

---

## Task 12: Connect AlertTopologyAnalysis Page

**Files:**
- Modify: `cmdb-demo/src/pages/AlertTopologyAnalysis.tsx`

This is the most complex frontend change — replacing fixed-position nodes with dynamic data + force layout.

- [ ] **Step 1: Read the current file**

Find `NODES` (hardcoded array with x,y coordinates) and `EDGES` (hardcoded connections). Understand how the SVG rendering uses `node.x, node.y` for positioning.

- [ ] **Step 2: Replace NODES/EDGES with API data**

1. Import `useTopologyGraph`, `useCreateDependency`, `useDeleteDependency` from `../hooks/useTopology`
2. Import `useForceLayout` from `../hooks/useForceLayout`
3. Get a location ID (from URL params, context, or default)
4. Fetch graph: `const { data: graphData } = useTopologyGraph(locationId)`
5. Extract: `const apiNodes = (graphData as any)?.nodes ?? []` and `const apiEdges = (graphData as any)?.edges ?? []`
6. Run layout: `const layoutNodes = useForceLayout(apiNodes, apiEdges, containerWidth, containerHeight)`
7. Map API node fields to the existing `TopologyNode` interface: `id, name, type, status, ip_address→ip, model, rack_name→rack, bia_level→biaLevel, has_active_alert, tags`
8. Derive `icon` from `type`: server→dns, network→router, storage→storage, power→bolt
9. Derive `cpu, memory, diskIO` from `metrics` object (or 0 if null)
10. Map API edge fields: `from, to, type, description, isFaultPath` — already matching
11. Delete the hardcoded `NODES` and `EDGES` arrays
12. Update SVG rendering to use `layoutNode.x, layoutNode.y` instead of hardcoded positions
13. If graph is empty, show "No assets in this location" message

- [ ] **Step 3: Add create/delete dependency buttons**

1. Add "Add Dependency" button that opens a simple form (source asset, target asset, type)
2. Wire to `useCreateDependency().mutate()`
3. Add delete button on edges (small X icon)
4. Wire to `useDeleteDependency().mutate()`

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/AlertTopologyAnalysis.tsx
git commit -m "feat: connect AlertTopology to dynamic graph API with force-directed layout"
```

---

## Task 13: Connect AuditEventDetail Page

**Files:**
- Modify: `cmdb-demo/src/pages/AuditEventDetail.tsx`

- [ ] **Step 1: Read the current file**

Find `FALLBACK_EVENT`, `FALLBACK_DIFF`, and the current `useAuditEvents(params)` workaround.

- [ ] **Step 2: Replace with dedicated hook**

1. Import `useAuditEventDetail` from `../hooks/useAudit`
2. Get event ID from URL params
3. Replace: `const { data: eventData } = useAuditEventDetail(eventId)`
4. Build event object from `(eventData as any)?.event` — use `operator_name`, `operator_email` directly
5. Parse `diff` from API response (it's a JSON string, may need `JSON.parse`)
6. Delete `FALLBACK_EVENT` and `FALLBACK_DIFF` constants
7. If event not found, show error state

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/AuditEventDetail.tsx
git commit -m "feat: connect AuditEventDetail to enriched audit event API"
```

---

## Task 14: Build Verification + Smoke Test

- [ ] **Step 1: Go build**

```bash
cd /cmdb-platform/cmdb-core && go build -o server ./cmd/server/
```

Expected: Clean build.

- [ ] **Step 2: TypeScript check**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit 2>&1 | grep -E "InventoryItem|MaintenanceTask|RackDetail|RackManagement|AlertTopology|AuditEvent|useForce|useActivity" | head -20
```

Expected: No errors in our changed files.

- [ ] **Step 3: Restart services and test endpoints**

```bash
# Restart cmdb-core
kill $(lsof -t -i:8080) 2>/dev/null; sleep 1
cd /cmdb-platform/cmdb-core && DATABASE_URL="postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable" NATS_URL="nats://localhost:4222" REDIS_URL="redis://localhost:6379/0" JWT_SECRET="changeme" nohup ./server > /tmp/cmdb-core.log 2>&1 &
sleep 3

# Get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('access_token',''))")

# Test new endpoints
curl -s "http://localhost:8080/api/v1/activity-feed?target_type=rack&target_id=00000000-0000-0000-0000-000000000001" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
curl -s "http://localhost:8080/api/v1/topology/graph?location_id=00000000-0000-0000-0000-000000000001" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

- [ ] **Step 4: Restart frontend**

```bash
kill $(lsof -t -i:5175) 2>/dev/null; sleep 1
cd /cmdb-platform/cmdb-demo && nohup npx vite > /tmp/cmdb-frontend.log 2>&1 &
sleep 3
```

- [ ] **Step 5: Fix any issues found during smoke test**
