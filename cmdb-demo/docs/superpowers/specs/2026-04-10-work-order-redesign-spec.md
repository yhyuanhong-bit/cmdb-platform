# Work Order Workflow Redesign Spec

**Date**: 2026-04-10
**Status**: Draft
**Scope**: `cmdb-core` backend + `cmdb-demo` frontend
**Migration**: `000025_work_order_redesign.up.sql`

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Simplified State Machine](#2-simplified-state-machine)
3. [Approval Rules](#3-approval-rules)
4. [Cross-Module Event Subscribers](#4-cross-module-event-subscribers)
5. [Notification Hooks](#5-notification-hooks)
6. [SLA Framework](#6-sla-framework)
7. [Database Changes](#7-database-changes)
8. [API Changes](#8-api-changes)
9. [Service Layer Changes](#9-service-layer-changes)
10. [Frontend Changes](#10-frontend-changes)
11. [Migration and Implementation Plan](#11-migration-and-implementation-plan)

---

## 1. Problem Statement

The current work order system has seven structural deficiencies that prevent it from functioning as a real ITSM workflow engine:

| # | Problem | Current Code Location | Impact |
|---|---------|----------------------|--------|
| 1 | Approval is a rubber stamp | `service.go:Transition` -- no role check, no self-approval block | Anyone can approve any order including their own |
| 2 | No closed-loop | `statemachine.go` -- `completed -> closed` does nothing | Work completion never updates assets, alerts, or inventory |
| 3 | Modules are isolated | No subscribers in `main.go` beyond WS bridge + webhook | Alerts, assets, inventory, predictions never react to WO changes |
| 4 | No notifications | Events publish to NATS but nobody listens | Users discover state changes only by polling the UI |
| 5 | draft -> pending is unnecessary | `statemachine.go` line 7-8 | Extra click for zero value -- every create should auto-submit |
| 6 | No SLA/timeout | No columns, no timer, no breach detection | Pending orders rot forever with no escalation |
| 7 | Upgrade WO bypasses service layer | `phase4_prediction_endpoints.go:407-410` -- raw `INSERT INTO work_orders` | Skips validation, event publishing, audit logging, code generation collision |

---

## 2. Simplified State Machine

### New States

```
submitted --> approved  --> in_progress --> completed --> verified
         \-> rejected (terminal)
```

| State | Meaning | Who Enters | Deletable |
|-------|---------|-----------|-----------|
| `submitted` | Created and awaiting approval | System (on create) | Yes |
| `approved` | Approved, ready to start work | Approver (ops-admin, super-admin) | No |
| `rejected` | Denied with reason | Approver | Yes (soft-delete) |
| `in_progress` | Technician actively working | Assignee | No |
| `completed` | Work done, awaiting verification | Assignee | No |
| `verified` | Verified by requestor/admin, fully closed | Requestor or admin | No |

### New Transition Map

```go
// internal/domain/maintenance/statemachine.go

package maintenance

import "fmt"

// Status constants to eliminate magic strings throughout the codebase.
const (
    StatusSubmitted  = "submitted"
    StatusApproved   = "approved"
    StatusRejected   = "rejected"
    StatusInProgress = "in_progress"
    StatusCompleted  = "completed"
    StatusVerified   = "verified"
)

// validTransitions defines the allowed status transitions for work orders.
var validTransitions = map[string][]string{
    StatusSubmitted:  {StatusApproved, StatusRejected},
    StatusApproved:   {StatusInProgress},
    StatusInProgress: {StatusCompleted},
    StatusCompleted:  {StatusVerified},
    // rejected and verified are terminal -- no outbound transitions
}

// ValidateTransition checks whether a transition from one status to another is allowed.
func ValidateTransition(from, to string) error {
    allowed, ok := validTransitions[from]
    if !ok {
        return fmt.Errorf("no transitions allowed from status %q", from)
    }
    for _, s := range allowed {
        if s == to {
            return nil
        }
    }
    return fmt.Errorf("invalid transition from %q to %q", from, to)
}
```

### Data Migration for Existing Rows

```sql
-- Migrate existing statuses to new model
UPDATE work_orders SET status = 'submitted' WHERE status IN ('draft', 'pending');
UPDATE work_orders SET status = 'verified'  WHERE status = 'closed';
-- 'approved', 'in_progress', 'completed', 'rejected' remain unchanged
```

---

## 3. Approval Rules

### 3.1 Who Can Approve

Only users whose resolved permissions include `maintenance:write` **and** who hold the role `ops-admin` or `super-admin`.

The check is performed inside the service layer, not the HTTP handler, so that all callers (API, event-driven auto-create, MCP) go through the same gate.

### 3.2 Self-Approval Block

The operator performing the `submitted -> approved` or `submitted -> rejected` transition must not be the same user as `requestor_id` on the work order.

### 3.3 Required Comments

- `submitted -> approved`: `comment` field is **required** (the approval reason).
- `submitted -> rejected`: `comment` field is **required** (the rejection reason).
- All other transitions: `comment` is optional.

### 3.4 Implementation

```go
// internal/domain/maintenance/service.go -- inside Transition()

// Approval guard: runs for submitted -> approved | rejected
func (s *Service) validateApproval(order dbgen.WorkOrder, operatorID uuid.UUID, operatorRoles []string, req TransitionRequest) error {
    if order.Status != StatusSubmitted {
        return nil // guard only applies when leaving submitted
    }

    // 1. Role check
    allowed := false
    for _, role := range operatorRoles {
        if role == "ops-admin" || role == "super-admin" {
            allowed = true
            break
        }
    }
    if !allowed {
        return fmt.Errorf("only ops-admin or super-admin can approve/reject work orders")
    }

    // 2. Self-approval block
    if order.RequestorID.Valid && order.RequestorID.Bytes == operatorID {
        return fmt.Errorf("cannot approve or reject your own work order")
    }

    // 3. Comment required
    if strings.TrimSpace(req.Comment) == "" {
        return fmt.Errorf("comment is required when approving or rejecting a work order")
    }

    return nil
}
```

### 3.5 Extended TransitionRequest

```go
// internal/domain/maintenance/model.go

type TransitionRequest struct {
    Status  string `json:"status"  binding:"required"`
    Comment string `json:"comment"`
}
```

No struct change needed -- `Comment` already exists. The service layer now enforces that it is non-empty for approval/rejection transitions.

### 3.6 Extended Transition Method Signature

The `Transition` method needs the operator's roles passed in so it can enforce the approval gate without importing the identity package or hitting the DB a second time. The API handler already has the roles from the RBAC middleware context.

```go
func (s *Service) Transition(
    ctx context.Context,
    tenantID, id, operatorID uuid.UUID,
    operatorRoles []string,       // NEW parameter
    req TransitionRequest,
) (*dbgen.WorkOrder, error) {
    order, err := s.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: id, TenantID: tenantID})
    if err != nil {
        return nil, fmt.Errorf("get work order: %w", err)
    }

    if err := ValidateTransition(order.Status, req.Status); err != nil {
        return nil, err
    }

    // Approval rules
    if err := s.validateApproval(order, operatorID, operatorRoles, req); err != nil {
        return nil, err
    }

    // SLA: stamp approved_at when entering approved state
    if req.Status == StatusApproved {
        now := time.Now()
        _ = s.queries.SetWorkOrderApprovedAt(ctx, dbgen.SetWorkOrderApprovedAtParams{
            ID: id, ApprovedAt: pgtype.Timestamptz{Time: now, Valid: true},
        })
    }

    // Record actual_start when entering in_progress
    if req.Status == StatusInProgress {
        now := time.Now()
        _ = s.queries.UpdateWorkOrderActualStart(ctx, dbgen.UpdateWorkOrderActualStartParams{
            ID: id, ActualStart: pgtype.Timestamptz{Time: now, Valid: true},
        })
    }

    // Record actual_end when entering completed
    if req.Status == StatusCompleted {
        now := time.Now()
        _ = s.queries.UpdateWorkOrderActualEnd(ctx, dbgen.UpdateWorkOrderActualEndParams{
            ID: id, ActualEnd: pgtype.Timestamptz{Time: now, Valid: true},
        })
    }

    updated, err := s.queries.UpdateWorkOrderStatus(ctx, dbgen.UpdateWorkOrderStatusParams{
        ID: id, Status: req.Status,
    })
    if err != nil {
        return nil, fmt.Errorf("update work order status: %w", err)
    }

    // Audit log
    logParams := dbgen.CreateWorkOrderLogParams{
        OrderID:    id,
        Action:     "transition",
        FromStatus: pgtype.Text{String: order.Status, Valid: true},
        ToStatus:   pgtype.Text{String: req.Status, Valid: true},
        OperatorID: pgtype.UUID{Bytes: operatorID, Valid: true},
    }
    if req.Comment != "" {
        logParams.Comment = pgtype.Text{String: req.Comment, Valid: true}
    }
    _, _ = s.queries.CreateWorkOrderLog(ctx, logParams)

    // Publish transition event (subscribers react asynchronously)
    if s.bus != nil {
        payload, _ := json.Marshal(map[string]string{
            "order_id":    id.String(),
            "tenant_id":   tenantID.String(),
            "from_status": order.Status,
            "to_status":   req.Status,
            "type":        order.Type,
            "priority":    order.Priority,
            "asset_id":    uuidToString(order.AssetID),
            "operator_id": operatorID.String(),
        })
        _ = s.bus.Publish(ctx, eventbus.Event{
            Subject:  eventbus.SubjectOrderTransitioned,
            TenantID: tenantID.String(),
            Payload:  payload,
        })
    }

    return &updated, nil
}
```

---

## 4. Cross-Module Event Subscribers

All subscribers are registered in `main.go` alongside the existing NATS-to-WebSocket bridge. Each subscriber is a standalone struct in a new package `internal/domain/workflows/` so that cross-domain orchestration does not pollute individual domain packages.

### 4.1 Package Structure

```
internal/domain/workflows/
    subscriber.go          -- WorkflowSubscriber struct, registration
    on_order_completed.go  -- handles maintenance.order_transitioned where to_status=completed
    on_order_verified.go   -- handles maintenance.order_transitioned where to_status=verified
    on_alert_fired.go      -- handles alert.fired
    on_discrepancy.go      -- handles inventory.item_scanned where status=discrepancy
```

### 4.2 Subscriber Struct

```go
// internal/domain/workflows/subscriber.go

package workflows

import (
    "context"
    "encoding/json"

    "github.com/cmdb-platform/cmdb-core/internal/dbgen"
    "github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
    "github.com/cmdb-platform/cmdb-core/internal/eventbus"
    "github.com/google/uuid"
    "go.uber.org/zap"
)

// WorkflowSubscriber reacts to domain events and orchestrates cross-module side effects.
type WorkflowSubscriber struct {
    queries        *dbgen.Queries
    maintenanceSvc *maintenance.Service
    bus            eventbus.Bus
}

func NewWorkflowSubscriber(
    queries *dbgen.Queries,
    maintenanceSvc *maintenance.Service,
    bus eventbus.Bus,
) *WorkflowSubscriber {
    return &WorkflowSubscriber{
        queries:        queries,
        maintenanceSvc: maintenanceSvc,
        bus:            bus,
    }
}

// Register subscribes to all relevant NATS subjects.
func (w *WorkflowSubscriber) Register() {
    if w.bus == nil {
        return
    }

    // React to work order transitions
    _ = w.bus.Subscribe(eventbus.SubjectOrderTransitioned, w.onOrderTransitioned)

    // React to alerts
    _ = w.bus.Subscribe(eventbus.SubjectAlertFired, w.onAlertFired)

    // React to inventory discrepancies
    _ = w.bus.Subscribe(eventbus.SubjectInventoryItemScanned, w.onInventoryItemScanned)

    zap.L().Info("workflow subscribers registered")
}
```

### 4.3 On Work Order Completed

When a work order reaches `completed`, the system applies downstream mutations depending on the order type.

```go
// internal/domain/workflows/on_order_completed.go

package workflows

func (w *WorkflowSubscriber) onOrderTransitioned(ctx context.Context, event eventbus.Event) error {
    var payload struct {
        OrderID    string `json:"order_id"`
        TenantID   string `json:"tenant_id"`
        FromStatus string `json:"from_status"`
        ToStatus   string `json:"to_status"`
        Type       string `json:"type"`
        Priority   string `json:"priority"`
        AssetID    string `json:"asset_id"`
        OperatorID string `json:"operator_id"`
    }
    if err := json.Unmarshal(event.Payload, &payload); err != nil {
        zap.L().Error("workflow: failed to unmarshal order transition", zap.Error(err))
        return nil // don't block the pipeline
    }

    switch payload.ToStatus {
    case "completed":
        return w.handleOrderCompleted(ctx, payload)
    case "verified":
        return w.handleOrderVerified(ctx, payload)
    default:
        return nil
    }
}

func (w *WorkflowSubscriber) handleOrderCompleted(ctx context.Context, p orderTransitionPayload) error {
    tenantID, _ := uuid.Parse(p.TenantID)
    assetID, _ := uuid.Parse(p.AssetID)

    switch p.Type {
    case "change_audit":
        // Update asset updated_at timestamp + create audit event
        if assetID != uuid.Nil {
            _ = w.queries.TouchAssetUpdatedAt(ctx, dbgen.TouchAssetUpdatedAtParams{
                ID: assetID, TenantID: tenantID,
            })
            zap.L().Info("workflow: change_audit completed, asset timestamp updated",
                zap.String("asset_id", p.AssetID))
        }

    case "upgrade":
        // The work order description or linked prediction carries the new spec.
        // Update asset attributes with the upgrade result.
        if assetID != uuid.Nil {
            orderID, _ := uuid.Parse(p.OrderID)
            order, err := w.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{
                ID: orderID, TenantID: tenantID,
            })
            if err == nil && order.Description.Valid {
                // Publish asset-updated event so other listeners (cache, WS) pick it up
                _ = w.bus.Publish(ctx, eventbus.Event{
                    Subject:  eventbus.SubjectAssetUpdated,
                    TenantID: p.TenantID,
                    Payload: mustJSON(map[string]string{
                        "asset_id":  p.AssetID,
                        "tenant_id": p.TenantID,
                        "source":    "work_order_completed",
                        "order_id":  p.OrderID,
                    }),
                })
            }
            zap.L().Info("workflow: upgrade completed, asset update triggered",
                zap.String("asset_id", p.AssetID))
        }
    }

    // Auto-resolve alerts linked to this asset
    if assetID != uuid.Nil {
        resolved, _ := w.queries.ResolveAlertsByAssetID(ctx, dbgen.ResolveAlertsByAssetIDParams{
            AssetID:  pgtype.UUID{Bytes: assetID, Valid: true},
            TenantID: tenantID,
        })
        for _, alert := range resolved {
            _ = w.bus.Publish(ctx, eventbus.Event{
                Subject:  eventbus.SubjectAlertResolved,
                TenantID: p.TenantID,
                Payload: mustJSON(map[string]string{
                    "alert_id": alert.ID.String(),
                    "asset_id": p.AssetID,
                    "source":   "work_order_completed",
                }),
            })
        }
        if len(resolved) > 0 {
            zap.L().Info("workflow: auto-resolved alerts on work order completion",
                zap.Int("count", len(resolved)),
                zap.String("asset_id", p.AssetID))
        }
    }

    // Auto-resolve inventory discrepancies linked to this asset
    if assetID != uuid.Nil {
        _ = w.queries.ResolveInventoryDiscrepanciesByAssetID(ctx, dbgen.ResolveInventoryDiscrepanciesByAssetIDParams{
            AssetID: pgtype.UUID{Bytes: assetID, Valid: true},
        })
    }

    return nil
}

func (w *WorkflowSubscriber) handleOrderVerified(ctx context.Context, p orderTransitionPayload) error {
    // verified is the terminal state -- nothing to trigger except a final audit event
    zap.L().Info("workflow: work order verified",
        zap.String("order_id", p.OrderID),
        zap.String("type", p.Type))
    return nil
}
```

### 4.4 On Critical Alert Fired

```go
// internal/domain/workflows/on_alert_fired.go

package workflows

func (w *WorkflowSubscriber) onAlertFired(ctx context.Context, event eventbus.Event) error {
    var payload struct {
        AlertID  string `json:"alert_id"`
        TenantID string `json:"tenant_id"`
        AssetID  string `json:"asset_id"`
        Severity string `json:"severity"`
        Message  string `json:"message"`
        RuleID   string `json:"rule_id"`
    }
    if err := json.Unmarshal(event.Payload, &payload); err != nil {
        return nil
    }

    // Only auto-create for critical alerts
    if payload.Severity != "critical" {
        return nil
    }

    tenantID, _ := uuid.Parse(payload.TenantID)

    // Check if an open emergency WO already exists for this asset to avoid duplicates
    assetUUID, _ := uuid.Parse(payload.AssetID)
    if assetUUID != uuid.Nil {
        existing, _ := w.queries.CountOpenOrdersByAssetAndType(ctx, dbgen.CountOpenOrdersByAssetAndTypeParams{
            TenantID: tenantID,
            AssetID:  pgtype.UUID{Bytes: assetUUID, Valid: true},
            Type:     "emergency",
        })
        if existing > 0 {
            zap.L().Debug("workflow: emergency WO already exists for asset, skipping",
                zap.String("asset_id", payload.AssetID))
            return nil
        }
    }

    // Use a system user UUID for auto-created orders
    systemUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

    assetIDPtr := &assetUUID
    if assetUUID == uuid.Nil {
        assetIDPtr = nil
    }

    _, err := w.maintenanceSvc.Create(ctx, tenantID, systemUserID, maintenance.CreateOrderRequest{
        Title:       fmt.Sprintf("Emergency: %s", payload.Message),
        Type:        "emergency",
        Priority:    "critical",
        AssetID:     assetIDPtr,
        Description: fmt.Sprintf("Auto-created from critical alert %s. %s", payload.AlertID, payload.Message),
        Reason:      "Critical alert auto-trigger",
    })
    if err != nil {
        zap.L().Error("workflow: failed to auto-create emergency work order",
            zap.String("alert_id", payload.AlertID), zap.Error(err))
        return nil // don't fail the event pipeline
    }

    zap.L().Info("workflow: auto-created emergency work order from critical alert",
        zap.String("alert_id", payload.AlertID),
        zap.String("asset_id", payload.AssetID))

    return nil
}
```

### 4.5 On Inventory Discrepancy Found

```go
// internal/domain/workflows/on_discrepancy.go

package workflows

func (w *WorkflowSubscriber) onInventoryItemScanned(ctx context.Context, event eventbus.Event) error {
    var payload struct {
        ItemID   string `json:"item_id"`
        TenantID string `json:"tenant_id"`
        AssetID  string `json:"asset_id"`
        Status   string `json:"status"`
        TaskID   string `json:"task_id"`
    }
    if err := json.Unmarshal(event.Payload, &payload); err != nil {
        return nil
    }

    // Only react to "missing" discrepancies
    if payload.Status != "missing" {
        return nil
    }

    tenantID, _ := uuid.Parse(payload.TenantID)
    assetUUID, _ := uuid.Parse(payload.AssetID)
    systemUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

    assetIDPtr := &assetUUID
    if assetUUID == uuid.Nil {
        assetIDPtr = nil
    }

    _, err := w.maintenanceSvc.Create(ctx, tenantID, systemUserID, maintenance.CreateOrderRequest{
        Title:       fmt.Sprintf("Investigate Missing Asset (inventory task %s)", payload.TaskID),
        Type:        "investigation",
        Priority:    "high",
        AssetID:     assetIDPtr,
        Description: fmt.Sprintf("Inventory scan found asset missing. Item ID: %s, Task ID: %s", payload.ItemID, payload.TaskID),
        Reason:      "Inventory discrepancy auto-trigger",
    })
    if err != nil {
        zap.L().Error("workflow: failed to auto-create investigation work order",
            zap.String("item_id", payload.ItemID), zap.Error(err))
    }

    return nil
}
```

### 4.6 Registration in main.go

Add after the existing webhook dispatcher block (~line 280):

```go
// Workflow subscribers (cross-module event reactions)
if bus != nil {
    wfSub := workflows.NewWorkflowSubscriber(queries, maintenanceSvc, bus)
    wfSub.Register()
}
```

---

## 5. Notification Hooks

### 5.1 Notification Model

Notifications are stored in a new `notifications` table and also pushed via WebSocket for real-time delivery.

```go
// internal/domain/workflows/notifications.go

package workflows

// NotificationType constants
const (
    NotifyApprovalRequired = "approval_required"
    NotifyOrderApproved    = "order_approved"
    NotifyOrderRejected    = "order_rejected"
    NotifyWorkCompleted    = "work_completed"
    NotifySLAWarning       = "sla_warning"
    NotifySLABreach        = "sla_breach"
)
```

### 5.2 Notification Rules by Transition

| Transition | Notification Type | Recipients | Payload |
|-----------|------------------|------------|---------|
| `-> submitted` | `approval_required` | All users with role `ops-admin` or `super-admin` in the same tenant | order_id, title, priority, requestor name |
| `-> approved` | `order_approved` | assignee_id + requestor_id | order_id, title, approver name, comment |
| `-> rejected` | `order_rejected` | requestor_id | order_id, title, rejector name, comment (reason) |
| `-> completed` | `work_completed` | requestor_id | order_id, title, message "please verify" |
| SLA 75% | `sla_warning` | assignee_id | order_id, title, remaining time |
| SLA 100% | `sla_breach` | assignee_id + first user with `super-admin` role | order_id, title, breach duration |

### 5.3 Notification Dispatch

Notifications are created in the DB and simultaneously pushed to WebSocket for real-time display:

```go
func (w *WorkflowSubscriber) notify(ctx context.Context, tenantID uuid.UUID, notifType string, recipientIDs []uuid.UUID, data map[string]string) {
    payload, _ := json.Marshal(data)

    for _, recipientID := range recipientIDs {
        // Persist to DB
        _ = w.queries.CreateNotification(ctx, dbgen.CreateNotificationParams{
            TenantID:    tenantID,
            RecipientID: recipientID,
            Type:        notifType,
            Payload:     payload,
        })
    }

    // Push to WebSocket for real-time delivery
    if w.bus != nil {
        wsPayload, _ := json.Marshal(map[string]any{
            "type":          notifType,
            "recipient_ids": recipientIDs,
            "data":          data,
        })
        _ = w.bus.Publish(ctx, eventbus.Event{
            Subject:  "notification.created",
            TenantID: tenantID.String(),
            Payload:  wsPayload,
        })
    }
}
```

### 5.4 New Event Subject

Add to `internal/eventbus/subjects.go`:

```go
SubjectNotificationCreated = "notification.created"
```

And add `"notification.>"` to the NATS-to-WebSocket bridge subjects in `main.go`.

---

## 6. SLA Framework

### 6.1 SLA Definitions by Priority

| Priority | Target Duration | Warning At | Breach At |
|----------|---------------|------------|-----------|
| critical | 4 hours | 3 hours (75%) | 4 hours |
| high | 8 hours | 6 hours (75%) | 8 hours |
| medium | 24 hours | 18 hours (75%) | 24 hours |
| low | 72 hours | 54 hours (75%) | 72 hours |

### 6.2 SLA Timer

- **Starts**: When work order enters `approved` state (`approved_at` column).
- **Pauses**: Never. Once approved, the clock runs. If the order is blocked, the assignee should note it in comments.
- **Stops**: When work order enters `completed` state (`actual_end` column).

### 6.3 SLA Columns

```sql
-- Added to work_orders table (see section 7)
approved_at      TIMESTAMPTZ,      -- set on approved transition
approved_by      UUID REFERENCES users(id),
sla_deadline     TIMESTAMPTZ,      -- computed: approved_at + SLA duration
sla_warning_sent BOOLEAN NOT NULL DEFAULT false,
sla_breached     BOOLEAN NOT NULL DEFAULT false,
```

### 6.4 SLA Computation in Service Layer

```go
// internal/domain/maintenance/sla.go

package maintenance

import "time"

// SLADuration returns the target resolution time for a given priority.
func SLADuration(priority string) time.Duration {
    switch priority {
    case "critical":
        return 4 * time.Hour
    case "high":
        return 8 * time.Hour
    case "medium":
        return 24 * time.Hour
    case "low":
        return 72 * time.Hour
    default:
        return 24 * time.Hour
    }
}

// SLAWarningDuration returns 75% of the SLA target.
func SLAWarningDuration(priority string) time.Duration {
    return SLADuration(priority) * 3 / 4
}

// SLADeadline computes the absolute deadline from an approval timestamp.
func SLADeadline(approvedAt time.Time, priority string) time.Time {
    return approvedAt.Add(SLADuration(priority))
}
```

### 6.5 SLA Checker (Background Goroutine)

```go
// internal/domain/workflows/sla_checker.go

package workflows

import (
    "context"
    "time"

    "go.uber.org/zap"
)

// StartSLAChecker runs a background loop that checks for SLA warnings and breaches.
// It runs every 60 seconds.
func (w *WorkflowSubscriber) StartSLAChecker(ctx context.Context) {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            w.checkSLAWarnings(ctx)
            w.checkSLABreaches(ctx)
        }
    }
}

func (w *WorkflowSubscriber) checkSLAWarnings(ctx context.Context) {
    // Query: approved or in_progress orders where
    //   now() > approved_at + (sla_duration * 0.75)
    //   AND sla_warning_sent = false
    orders, err := w.queries.ListSLAWarningCandidates(ctx)
    if err != nil {
        zap.L().Error("sla: failed to query warning candidates", zap.Error(err))
        return
    }

    for _, order := range orders {
        // Send warning notification to assignee
        if order.AssigneeID.Valid {
            w.notify(ctx, order.TenantID, NotifySLAWarning, []uuid.UUID{
                order.AssigneeID.Bytes,
            }, map[string]string{
                "order_id":     order.ID.String(),
                "title":        order.Title,
                "priority":     order.Priority,
                "sla_deadline": order.SlaDeadline.Time.Format(time.RFC3339),
            })
        }

        // Mark warning as sent
        _ = w.queries.MarkSLAWarningSent(ctx, order.ID)
    }
}

func (w *WorkflowSubscriber) checkSLABreaches(ctx context.Context) {
    // Query: approved or in_progress orders where
    //   now() > sla_deadline
    //   AND sla_breached = false
    orders, err := w.queries.ListSLABreachCandidates(ctx)
    if err != nil {
        zap.L().Error("sla: failed to query breach candidates", zap.Error(err))
        return
    }

    for _, order := range orders {
        recipients := []uuid.UUID{}
        if order.AssigneeID.Valid {
            recipients = append(recipients, order.AssigneeID.Bytes)
        }

        // Escalate to super-admins
        admins, _ := w.queries.ListUsersByRole(ctx, dbgen.ListUsersByRoleParams{
            TenantID: order.TenantID,
            RoleName: "super-admin",
        })
        for _, admin := range admins {
            recipients = append(recipients, admin.ID)
        }

        w.notify(ctx, order.TenantID, NotifySLABreach, recipients, map[string]string{
            "order_id":     order.ID.String(),
            "title":        order.Title,
            "priority":     order.Priority,
            "sla_deadline": order.SlaDeadline.Time.Format(time.RFC3339),
        })

        _ = w.queries.MarkSLABreached(ctx, order.ID)
    }
}
```

### 6.6 SLA Checker Registration in main.go

```go
// Start SLA checker background goroutine
if bus != nil {
    wfSub := workflows.NewWorkflowSubscriber(queries, maintenanceSvc, bus)
    wfSub.Register()
    go wfSub.StartSLAChecker(ctx)
}
```

---

## 7. Database Changes

### 7.1 Migration: `000025_work_order_redesign.up.sql`

```sql
-- =============================================================================
-- Migration: Work Order Workflow Redesign
-- =============================================================================

-- 1. Add new columns to work_orders
ALTER TABLE work_orders
    ADD COLUMN IF NOT EXISTS approved_at      TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS approved_by      UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS sla_deadline     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sla_warning_sent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS sla_breached     BOOLEAN NOT NULL DEFAULT false;

-- 2. Migrate existing statuses
UPDATE work_orders SET status = 'submitted' WHERE status IN ('draft', 'pending');
UPDATE work_orders SET status = 'verified'  WHERE status = 'closed';

-- 3. Add check constraint for valid statuses
ALTER TABLE work_orders
    ADD CONSTRAINT chk_work_order_status
    CHECK (status IN ('submitted', 'approved', 'rejected', 'in_progress', 'completed', 'verified'));

-- 4. Index for SLA checker queries
CREATE INDEX idx_work_orders_sla_deadline
    ON work_orders(sla_deadline)
    WHERE status IN ('approved', 'in_progress')
      AND sla_breached = false;

-- 5. Notifications table
CREATE TABLE notifications (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID         NOT NULL REFERENCES tenants(id),
    recipient_id UUID         NOT NULL REFERENCES users(id),
    type         VARCHAR(50)  NOT NULL,
    payload      JSONB        NOT NULL DEFAULT '{}',
    read         BOOLEAN      NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_recipient_unread
    ON notifications(recipient_id, created_at DESC)
    WHERE read = false;

CREATE INDEX idx_notifications_tenant
    ON notifications(tenant_id, created_at DESC);

-- 6. Add created_by column (currently the schema uses requestor_id,
--    but AcceptUpgradeRecommendation uses created_by -- normalize to requestor_id)
-- No change needed: requestor_id already exists. Fix the raw SQL in
-- AcceptUpgradeRecommendation to use requestor_id instead of created_by.
```

### 7.2 Down Migration: `000025_work_order_redesign.down.sql`

```sql
ALTER TABLE work_orders DROP CONSTRAINT IF EXISTS chk_work_order_status;

UPDATE work_orders SET status = 'draft'  WHERE status = 'submitted';
UPDATE work_orders SET status = 'closed' WHERE status = 'verified';

ALTER TABLE work_orders
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS sla_deadline,
    DROP COLUMN IF EXISTS sla_warning_sent,
    DROP COLUMN IF EXISTS sla_breached;

DROP TABLE IF EXISTS notifications;
DROP INDEX IF EXISTS idx_work_orders_sla_deadline;
```

### 7.3 New SQL Queries (sqlc)

Add to `db/queries/work_orders.sql`:

```sql
-- name: SetWorkOrderApprovedAt :exec
UPDATE work_orders
SET approved_at = @approved_at, approved_by = @approved_by, sla_deadline = @sla_deadline, updated_at = now()
WHERE id = @id;

-- name: UpdateWorkOrderActualStart :exec
UPDATE work_orders SET actual_start = @actual_start, updated_at = now() WHERE id = @id;

-- name: UpdateWorkOrderActualEnd :exec
UPDATE work_orders SET actual_end = @actual_end, updated_at = now() WHERE id = @id;

-- name: MarkSLAWarningSent :exec
UPDATE work_orders SET sla_warning_sent = true WHERE id = @id;

-- name: MarkSLABreached :exec
UPDATE work_orders SET sla_breached = true WHERE id = @id;

-- name: ListSLAWarningCandidates :many
SELECT * FROM work_orders
WHERE status IN ('approved', 'in_progress')
  AND sla_deadline IS NOT NULL
  AND sla_warning_sent = false
  AND now() > sla_deadline - (sla_deadline - approved_at) * 0.25;

-- name: ListSLABreachCandidates :many
SELECT * FROM work_orders
WHERE status IN ('approved', 'in_progress')
  AND sla_deadline IS NOT NULL
  AND sla_breached = false
  AND now() > sla_deadline;

-- name: CountOpenOrdersByAssetAndType :one
SELECT count(*) FROM work_orders
WHERE tenant_id = @tenant_id
  AND asset_id = @asset_id
  AND type = @type
  AND status NOT IN ('completed', 'verified', 'rejected');

-- name: TouchAssetUpdatedAt :exec
UPDATE assets SET updated_at = now() WHERE id = @id AND tenant_id = @tenant_id;

-- name: ResolveAlertsByAssetID :many
UPDATE alert_events
SET resolved_at = now(), status = 'resolved'
WHERE asset_id = @asset_id
  AND tenant_id = @tenant_id
  AND status = 'firing'
RETURNING *;

-- name: ResolveInventoryDiscrepanciesByAssetID :exec
UPDATE inventory_items
SET status = 'resolved'
WHERE asset_id = @asset_id
  AND status IN ('missing', 'mismatch');

-- name: CreateNotification :exec
INSERT INTO notifications (tenant_id, recipient_id, type, payload)
VALUES (@tenant_id, @recipient_id, @type, @payload);

-- name: ListUserNotifications :many
SELECT * FROM notifications
WHERE recipient_id = @recipient_id
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications
WHERE recipient_id = @recipient_id AND read = false;

-- name: MarkNotificationRead :exec
UPDATE notifications SET read = true WHERE id = @id AND recipient_id = @recipient_id;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET read = true WHERE recipient_id = @recipient_id AND read = false;

-- name: ListUsersByRole :many
SELECT u.* FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE r.tenant_id = @tenant_id AND r.name = @role_name;
```

---

## 8. API Changes

### 8.1 Modified Endpoints

#### `POST /maintenance/orders` -- Create Work Order

**Change**: Initial status is now `submitted` instead of `draft`. No separate "submit" step.

```go
// In service.go Create():
params.Status = "submitted"  // was "draft"
```

The initial log entry also changes:

```go
_, _ = s.queries.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
    OrderID:    order.ID,
    Action:     "submitted",           // was "created"
    ToStatus:   pgtype.Text{String: StatusSubmitted, Valid: true},
    OperatorID: pgtype.UUID{Bytes: requestorID, Valid: true},
})
```

#### `POST /maintenance/orders/{id}/transition` -- Transition Work Order

**Change**: Handler must pass `operatorRoles` from the gin context to the service.

```go
func (s *APIServer) TransitionWorkOrder(c *gin.Context, id IdPath) {
    var body struct {
        Status  string `json:"status" binding:"required"`
        Comment string `json:"comment"`
    }
    if err := c.ShouldBindJSON(&body); err != nil {
        response.BadRequest(c, err.Error())
        return
    }

    operatorID := userIDFromContext(c)
    operatorRoles := rolesFromContext(c)   // NEW: extract from RBAC middleware context

    order, err := s.maintenanceSvc.Transition(
        c.Request.Context(),
        tenantIDFromContext(c),
        uuid.UUID(id),
        operatorID,
        operatorRoles,                     // NEW parameter
        maintenance.TransitionRequest{
            Status:  body.Status,
            Comment: body.Comment,
        },
    )
    if err != nil {
        response.BadRequest(c, err.Error())
        return
    }

    s.recordAudit(c, "order.transitioned", "maintenance", "work_order", uuid.UUID(id), map[string]any{
        "from": body.Status, "to": order.Status, "comment": body.Comment,
    })
    response.OK(c, order)
}
```

#### `DELETE /maintenance/orders/{id}` -- Delete Work Order

**Change**: Allow deletion of `submitted` (was `draft`) and `rejected` orders.

```go
// In service.go Delete():
if order.Status != StatusSubmitted && order.Status != StatusRejected {
    return fmt.Errorf("cannot delete work order in '%s' status; only submitted or rejected orders can be deleted", order.Status)
}
```

### 8.2 New Endpoints

#### `GET /notifications`

List notifications for the authenticated user.

**Query params**: `unread_only` (boolean), `limit` (int, default 20), `offset` (int, default 0)

**Response**:
```json
{
  "data": [
    {
      "id": "uuid",
      "type": "approval_required",
      "payload": { "order_id": "uuid", "title": "...", "priority": "high" },
      "read": false,
      "created_at": "2026-04-10T12:00:00Z"
    }
  ],
  "meta": { "total": 5, "unread": 3 }
}
```

#### `POST /notifications/{id}/read`

Mark a single notification as read.

#### `POST /notifications/read-all`

Mark all notifications as read for the authenticated user.

#### `GET /maintenance/orders/{id}/sla`

Return SLA status for a specific work order.

**Response**:
```json
{
  "order_id": "uuid",
  "priority": "high",
  "sla_duration_hours": 8,
  "approved_at": "2026-04-10T08:00:00Z",
  "sla_deadline": "2026-04-10T16:00:00Z",
  "elapsed_hours": 5.2,
  "remaining_hours": 2.8,
  "percentage_used": 65,
  "status": "on_track",
  "warning_sent": false,
  "breached": false
}
```

Where `status` is one of: `not_started` (not yet approved), `on_track`, `warning`, `breached`.

### 8.3 Fix: AcceptUpgradeRecommendation

Replace the raw SQL insert in `phase4_prediction_endpoints.go` with a call through the maintenance service:

```go
func (s *APIServer) AcceptUpgradeRecommendation(c *gin.Context) {
    assetID := c.Param("id")
    category := c.Param("category")
    tenantID := tenantIDFromContext(c)
    userID := userIDFromContext(c)

    // Get the matching rule
    var ruleRecommendation string
    var assetType string
    err := s.pool.QueryRow(c.Request.Context(), `
        SELECT ur.recommendation, a.type
        FROM upgrade_rules ur
        JOIN assets a ON a.type = ur.asset_type AND a.tenant_id = ur.tenant_id
        WHERE a.id = $1 AND ur.category = $2 AND ur.tenant_id = $3 AND ur.enabled = true
        LIMIT 1
    `, assetID, category, tenantID).Scan(&ruleRecommendation, &assetType)
    if err != nil {
        response.NotFound(c, "no matching upgrade rule found")
        return
    }

    assetUUID, _ := uuid.Parse(assetID)
    title := fmt.Sprintf("Upgrade %s %s: %s", assetType, category, ruleRecommendation)

    order, err := s.maintenanceSvc.Create(c.Request.Context(), tenantID, userID, maintenance.CreateOrderRequest{
        Title:       title,
        Type:        "upgrade",
        Priority:    "medium",
        AssetID:     &assetUUID,
        Description: ruleRecommendation,
    })
    if err != nil {
        response.InternalError(c, "failed to create work order")
        return
    }

    s.recordAudit(c, "order.created", "maintenance", "work_order", order.ID, map[string]any{
        "source": "upgrade_recommendation", "category": category,
    })

    response.Created(c, gin.H{
        "work_order_id": order.ID.String(),
        "code":          order.Code,
    })
}
```

---

## 9. Service Layer Changes

### 9.1 Updated Create Method

```go
func (s *Service) Create(ctx context.Context, tenantID, requestorID uuid.UUID, req CreateOrderRequest) (*dbgen.WorkOrder, error) {
    priority := req.Priority
    if priority == "" {
        priority = "medium"
    }

    params := dbgen.CreateWorkOrderParams{
        TenantID:    tenantID,
        Code:        generateCode(),
        Title:       req.Title,
        Type:        req.Type,
        Status:      StatusSubmitted,   // was "draft"
        Priority:    priority,
        RequestorID: pgtype.UUID{Bytes: requestorID, Valid: true},
    }

    // ... existing field assignments unchanged ...

    order, err := s.queries.CreateWorkOrder(ctx, params)
    if err != nil {
        return nil, fmt.Errorf("create work order: %w", err)
    }

    // Log entry
    _, _ = s.queries.CreateWorkOrderLog(ctx, dbgen.CreateWorkOrderLogParams{
        OrderID:    order.ID,
        Action:     "submitted",
        ToStatus:   pgtype.Text{String: StatusSubmitted, Valid: true},
        OperatorID: pgtype.UUID{Bytes: requestorID, Valid: true},
    })

    // Publish creation event
    if s.bus != nil {
        payload, _ := json.Marshal(map[string]string{
            "order_id":   order.ID.String(),
            "tenant_id":  tenantID.String(),
            "title":      order.Title,
            "type":       order.Type,
            "priority":   order.Priority,
            "status":     StatusSubmitted,
            "asset_id":   uuidToString(order.AssetID),
        })
        _ = s.bus.Publish(ctx, eventbus.Event{
            Subject:  eventbus.SubjectOrderCreated,
            TenantID: tenantID.String(),
            Payload:  payload,
        })
    }

    return &order, nil
}
```

### 9.2 Code Generation Fix

The current `generateCode()` uses `rand.Intn(9000)` which can produce collisions. Replace with a DB sequence or atomic counter:

```go
func (s *Service) generateCode(ctx context.Context) string {
    year := time.Now().Year()
    prefix := fmt.Sprintf("WO-%d-", year)

    var maxCode *string
    _ = s.queries.GetMaxWorkOrderCode(ctx, prefix+"%").Scan(&maxCode)

    nextNum := 1
    if maxCode != nil {
        var n int
        fmt.Sscanf(*maxCode, prefix+"%d", &n)
        nextNum = n + 1
    }
    return fmt.Sprintf("%s%04d", prefix, nextNum)
}
```

---

## 10. Frontend Changes

### 10.1 Work Order List Page

| Change | Detail |
|--------|--------|
| Remove "draft" and "pending" filter chips | Replace with single "submitted" chip |
| Remove "closed" filter chip | Replace with "verified" |
| Add SLA indicator column | Show colored badge: green (on track), yellow (warning), red (breached) |
| Add reject button on list rows | Visible only for `submitted` status + user has approver role |

### 10.2 Work Order Detail Page

| Change | Detail |
|--------|--------|
| Approval section | Show "Approve" and "Reject" buttons when status is `submitted` and user is ops-admin/super-admin and user is not the requestor |
| Comment requirement | When clicking Approve or Reject, show a modal that requires a comment before submission |
| SLA panel | Show countdown timer: time remaining, percentage bar, deadline timestamp |
| Verification button | Show "Verify" button when status is `completed` and user is requestor or admin |
| Activity timeline | Already exists (work_order_logs). No change needed. |

### 10.3 Notification Bell

| Change | Detail |
|--------|--------|
| Header bell icon | Show unread count badge |
| Dropdown panel | List recent notifications, click to navigate to relevant work order |
| Mark as read | Click notification or "mark all read" button |
| WebSocket integration | Listen for `notification.created` events, increment badge count in real time |

### 10.4 Dashboard Widget

| Change | Detail |
|--------|--------|
| SLA compliance widget | Pie chart: on-track vs warning vs breached for active work orders |
| Pending approvals count | Badge on sidebar nav for users with approver role |

### 10.5 Status Badge Color Map

```typescript
const STATUS_COLORS: Record<string, string> = {
  submitted:   'blue',
  approved:    'green',
  rejected:    'red',
  in_progress: 'orange',
  completed:   'purple',
  verified:    'gray',
};
```

---

## 11. Migration and Implementation Plan

### Phase A: Database + State Machine (1 day)

1. Write and test migration `000025_work_order_redesign.up.sql` and down migration.
2. Replace `statemachine.go` with new transition map and status constants.
3. Run `sqlc generate` after adding new queries.
4. Update `model.go` -- no struct changes needed (TransitionRequest is unchanged).

**Files touched**:
- `db/migrations/000025_work_order_redesign.up.sql`
- `db/migrations/000025_work_order_redesign.down.sql`
- `db/queries/work_orders.sql`
- `internal/domain/maintenance/statemachine.go`

### Phase B: Service Layer (1 day)

1. Add `sla.go` with duration functions.
2. Update `service.go`:
   - `Create()` uses `StatusSubmitted`, publishes event.
   - `Transition()` accepts `operatorRoles`, calls `validateApproval()`, stamps SLA columns on approval, stamps `actual_start`/`actual_end`.
   - `Delete()` checks `StatusSubmitted` instead of `"draft"`.
3. Fix `generateCode()` to use DB max instead of random.

**Files touched**:
- `internal/domain/maintenance/service.go`
- `internal/domain/maintenance/sla.go` (new)

### Phase C: Workflow Subscribers (1 day)

1. Create `internal/domain/workflows/` package.
2. Implement `subscriber.go`, `on_order_completed.go`, `on_alert_fired.go`, `on_discrepancy.go`, `notifications.go`, `sla_checker.go`.
3. Register in `main.go`.
4. Add `SubjectNotificationCreated` to event subjects.
5. Add `"notification.>"` to NATS-to-WebSocket bridge.

**Files touched**:
- `internal/domain/workflows/` (new package, 6 files)
- `cmd/server/main.go`
- `internal/eventbus/subjects.go`

### Phase D: API Layer (0.5 day)

1. Update `TransitionWorkOrder` handler to pass `operatorRoles`.
2. Fix `AcceptUpgradeRecommendation` to use service layer.
3. Add notification endpoints: `GET /notifications`, `POST /notifications/{id}/read`, `POST /notifications/read-all`.
4. Add `GET /maintenance/orders/{id}/sla`.

**Files touched**:
- `internal/api/impl.go`
- `internal/api/phase4_prediction_endpoints.go`
- `internal/api/notification_endpoints.go` (new)

### Phase E: Frontend (1.5 days)

1. Update status filter chips and color map.
2. Add approval modal with required comment field.
3. Add reject button with required reason field.
4. Add SLA indicator to list and detail views.
5. Add notification bell component.
6. Add verification button on completed orders.
7. Wire WebSocket listener for real-time notification count.

### Phase F: Testing (1 day)

1. Unit tests for `validateApproval()` -- role check, self-approval block, comment required.
2. Unit tests for `ValidateTransition()` -- all valid and invalid paths.
3. Unit tests for SLA duration computation.
4. Integration test for workflow subscribers -- mock event bus, verify side effects.
5. Integration test for notification creation and WebSocket delivery.
6. Manual E2E test: create order -> approve -> start -> complete -> verify, verifying each notification fires.

**Total estimated effort**: 6 days

---

## Appendix A: Helper Utilities

```go
// internal/domain/workflows/helpers.go

package workflows

import "encoding/json"

func mustJSON(v any) json.RawMessage {
    b, _ := json.Marshal(v)
    return b
}

type orderTransitionPayload struct {
    OrderID    string `json:"order_id"`
    TenantID   string `json:"tenant_id"`
    FromStatus string `json:"from_status"`
    ToStatus   string `json:"to_status"`
    Type       string `json:"type"`
    Priority   string `json:"priority"`
    AssetID    string `json:"asset_id"`
    OperatorID string `json:"operator_id"`
}
```

## Appendix B: New Event Subjects Summary

```go
// Add to internal/eventbus/subjects.go
SubjectNotificationCreated = "notification.created"
```

No new maintenance subjects needed -- `SubjectOrderCreated`, `SubjectOrderUpdated`, and `SubjectOrderTransitioned` already exist and carry the full payload needed by subscribers.

## Appendix C: RBAC Context Helper

The RBAC middleware already stores resolved roles in the gin context. Add a helper to extract them:

```go
// internal/api/context.go (or wherever tenantIDFromContext lives)

func rolesFromContext(c *gin.Context) []string {
    roles, _ := c.Get("user_roles")
    if r, ok := roles.([]string); ok {
        return r
    }
    return nil
}
```
