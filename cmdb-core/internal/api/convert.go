package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func pguuidToPtr(v pgtype.UUID) *string {
	if !v.Valid {
		return nil
	}
	s := fmt.Sprintf("%x-%x-%x-%x-%x", v.Bytes[0:4], v.Bytes[4:6], v.Bytes[6:8], v.Bytes[8:10], v.Bytes[10:16])
	return &s
}

func pguuidToStr(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", v.Bytes[0:4], v.Bytes[4:6], v.Bytes[6:8], v.Bytes[8:10], v.Bytes[10:16])
}

func pgtextToPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func pgtextToStr(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func timeToStr(v time.Time) string {
	return v.Format(time.RFC3339)
}

func pgtsToPtr(v pgtype.Timestamptz) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.Format(time.RFC3339)
	return &s
}

func pgtsToTime(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time
}

func pgtsToTimePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func pgnumToPtr(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	val := f.Float64
	return &val
}

func float32ToNumeric(f float32) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}

func pgnumToFloat32(v pgtype.Numeric) float32 {
	if !v.Valid {
		return 0
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return float32(f.Float64)
}

func pgboolVal(v pgtype.Bool) bool {
	if !v.Valid {
		return false
	}
	return v.Bool
}

func pgboolToPtr(v pgtype.Bool) *bool {
	if !v.Valid {
		return nil
	}
	return &v.Bool
}

func pgdateToPtr(v pgtype.Date) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.Format("2006-01-02")
	return &s
}

func pgdateToStr(v pgtype.Date) string {
	if !v.Valid {
		return ""
	}
	return v.Time.Format("2006-01-02")
}

func bytesToJSON(b []byte) *map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return &m
}

func bytesToJSONVal(b []byte) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func rawJSONToMap(b json.RawMessage) *map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return &m
}

func rawJSONToMapVal(b json.RawMessage) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func rawJSONToPermissions(b json.RawMessage) map[string][]string {
	if len(b) == 0 {
		return nil
	}
	var m map[string][]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func convertSlice[F any, T any](items []F, fn func(F) T) []T {
	if items == nil {
		return nil
	}
	out := make([]T, len(items))
	for i, item := range items {
		out[i] = fn(item)
	}
	return out
}

// ---------------------------------------------------------------------------
// 1. toAPIAsset
// ---------------------------------------------------------------------------

func toAPIAsset(db dbgen.Asset) Asset {
	return Asset{
		Id:             db.ID,
		AssetTag:       db.AssetTag,
		PropertyNumber: pgtextToPtr(db.PropertyNumber),
		ControlNumber:  pgtextToPtr(db.ControlNumber),
		Name:           db.Name,
		Type:           db.Type,
		SubType:        pgtextToStr(db.SubType),
		Status:         db.Status,
		BiaLevel:       db.BiaLevel,
		LocationId:     pguuidToUUIDPtr(db.LocationID),
		RackId:         pguuidToUUIDPtr(db.RackID),
		Vendor:         pgtextToStr(db.Vendor),
		Model:          pgtextToStr(db.Model),
		SerialNumber:   pgtextToStr(db.SerialNumber),
		Attributes:     rawJSONToMapVal(db.Attributes),
		Tags:           db.Tags,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}
}

// pguuidToUUIDPtr converts a pgtype.UUID to *uuid.UUID (compatible with openapi_types.UUID).
func pguuidToUUIDPtr(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	u := uuid.UUID(v.Bytes)
	return &u
}

// ---------------------------------------------------------------------------
// 2. toAPILocation
// ---------------------------------------------------------------------------

func toAPILocation(db dbgen.Location) Location {
	return Location{
		Id:        db.ID,
		Name:      db.Name,
		NameEn:    pgtextToStr(db.NameEn),
		Slug:      db.Slug,
		Level:     db.Level,
		ParentId:  pguuidToUUIDPtr(db.ParentID),
		Path:      pgtextToStr(db.Path),
		Status:    db.Status,
		Metadata:  rawJSONToMapVal(db.Metadata),
		SortOrder: int(db.SortOrder),
		CreatedAt: db.CreatedAt,
		UpdatedAt: db.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// 3. toAPIRack
// ---------------------------------------------------------------------------

func toAPIRack(db dbgen.Rack) Rack {
	return Rack{
		Id:              db.ID,
		LocationId:      db.LocationID,
		Name:            db.Name,
		RowLabel:        pgtextToStr(db.RowLabel),
		TotalU:          int(db.TotalU),
		PowerCapacityKw: pgnumToFloat32(db.PowerCapacityKw),
		Status:          db.Status,
		Tags:            db.Tags,
		CreatedAt:       db.CreatedAt,
		// PowerCurrentKw and UsedU are computed fields, default to zero.
	}
}

// ---------------------------------------------------------------------------
// 4. toAPIWorkOrder
// ---------------------------------------------------------------------------

func toAPIWorkOrder(db dbgen.WorkOrder) WorkOrder {
	return WorkOrder{
		Id:             db.ID,
		Code:           db.Code,
		Title:          db.Title,
		Type:           db.Type,
		Status:         db.Status,
		Priority:       db.Priority,
		LocationId:     pgUUIDToUUID(db.LocationID),
		AssigneeId:     pguuidToUUIDPtr(db.AssigneeID),
		Description:    pgtextToStr(db.Description),
		ScheduledStart: pgtsToTime(db.ScheduledStart),
		ScheduledEnd:   pgtsToTime(db.ScheduledEnd),
		ActualStart:    pgtsToTimePtr(db.ActualStart),
		ActualEnd:      pgtsToTimePtr(db.ActualEnd),
		CreatedAt:      db.CreatedAt,
	}
}

// pgUUIDToUUID converts a pgtype.UUID to a uuid.UUID, returning a zero UUID if not valid.
func pgUUIDToUUID(v pgtype.UUID) uuid.UUID {
	if !v.Valid {
		return uuid.UUID{}
	}
	return uuid.UUID(v.Bytes)
}

// ---------------------------------------------------------------------------
// 5. toAPIWorkOrderLog
// ---------------------------------------------------------------------------

func toAPIWorkOrderLog(db dbgen.WorkOrderLog) WorkOrderLog {
	return WorkOrderLog{
		Id:         db.ID,
		OrderId:    db.OrderID,
		Action:     db.Action,
		FromStatus: pgtextToPtr(db.FromStatus),
		ToStatus:   pgtextToPtr(db.ToStatus),
		OperatorId: pguuidToUUIDPtr(db.OperatorID),
		Comment:    pgtextToPtr(db.Comment),
		CreatedAt:  db.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// 6. toAPIAlertEvent
// ---------------------------------------------------------------------------

func toAPIAlertEvent(db dbgen.AlertEvent) AlertEvent {
	return AlertEvent{
		Id:           db.ID,
		RuleId:       pgUUIDToUUID(db.RuleID),
		CiId:         pgUUIDToUUID(db.AssetID),
		Status:       db.Status,
		Severity:     db.Severity,
		Message:      pgtextToStr(db.Message),
		TriggerValue: pgnumToFloat32(db.TriggerValue),
		FiredAt:      db.FiredAt,
		ResolvedAt:   pgtsToTimePtr(db.ResolvedAt),
	}
}

// ---------------------------------------------------------------------------
// 7. toAPIInventoryTask
// ---------------------------------------------------------------------------

func toAPIInventoryTask(db dbgen.InventoryTask) InventoryTask {
	return InventoryTask{
		Id:              db.ID,
		Code:            db.Code,
		Name:            db.Name,
		ScopeLocationId: pgUUIDToUUID(db.ScopeLocationID),
		Status:          db.Status,
		Method:          pgtextToStr(db.Method),
		PlannedDate:     pgdateToStr(db.PlannedDate),
		CompletedDate:   pgdateToPtr(db.CompletedDate),
		AssignedTo:      pgUUIDToUUID(db.AssignedTo),
	}
}

// ---------------------------------------------------------------------------
// 8. toAPIInventoryItem
// ---------------------------------------------------------------------------

func toAPIInventoryItem(db dbgen.InventoryItem) InventoryItem {
	return InventoryItem{
		Id:        db.ID,
		TaskId:    db.TaskID,
		AssetId:   pguuidToUUIDPtr(db.AssetID),
		RackId:    pguuidToUUIDPtr(db.RackID),
		Expected:  bytesToJSONVal(db.Expected),
		Actual:    bytesToJSON(db.Actual),
		Status:    db.Status,
		ScannedAt: pgtsToTimePtr(db.ScannedAt),
		ScannedBy: pguuidToUUIDPtr(db.ScannedBy),
	}
}

// ---------------------------------------------------------------------------
// 9. toAPIAuditEvent
// ---------------------------------------------------------------------------

func toAPIAuditEvent(db dbgen.AuditEvent) AuditEvent {
	return AuditEvent{
		Id:         db.ID,
		Action:     db.Action,
		Module:     pgtextToStr(db.Module),
		TargetType: pgtextToStr(db.TargetType),
		TargetId:   pgUUIDToUUID(db.TargetID),
		OperatorId: pgUUIDToUUID(db.OperatorID),
		Diff:       bytesToJSONVal(db.Diff),
		CreatedAt:  db.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// 10. toAPIUser (excludes password_hash)
// ---------------------------------------------------------------------------

func toAPIUser(db dbgen.User) User {
	return User{
		Id:          db.ID,
		DisplayName: db.DisplayName,
		Email:       db.Email,
		Phone:       pgtextToStr(db.Phone),
		Username:    db.Username,
		Status:      db.Status,
		Source:      db.Source,
		CreatedAt:   db.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// 11. toAPIRole
// ---------------------------------------------------------------------------

func toAPIRole(db dbgen.Role) Role {
	return Role{
		Id:          db.ID,
		Name:        db.Name,
		Description: pgtextToStr(db.Description),
		Permissions: rawJSONToPermissions(db.Permissions),
		IsSystem:    db.IsSystem,
	}
}

// ---------------------------------------------------------------------------
// 12. toAPIPredictionModel
// ---------------------------------------------------------------------------

func toAPIPredictionModel(db dbgen.PredictionModel) PredictionModel {
	return PredictionModel{
		Id:       db.ID,
		Name:     db.Name,
		Type:     db.Type,
		Provider: db.Provider,
		Config:   rawJSONToMapVal(db.Config),
		Enabled:  pgboolVal(db.Enabled),
	}
}

// ---------------------------------------------------------------------------
// 13. toAPIPredictionResult
// ---------------------------------------------------------------------------

func toAPIPredictionResult(db dbgen.PredictionResult) PredictionResult {
	return PredictionResult{
		Id:                db.ID,
		CiId:              db.AssetID,
		ModelId:           db.ModelID,
		PredictionType:    db.PredictionType,
		Result:            rawJSONToMapVal(db.Result),
		Severity:          pgtextToStr(db.Severity),
		RecommendedAction: pgtextToStr(db.RecommendedAction),
		ExpiresAt:         pgtsToTime(db.ExpiresAt),
		CreatedAt:         pgtsToTime(db.CreatedAt),
	}
}

// ---------------------------------------------------------------------------
// 14. toAPIRCAAnalysis
// ---------------------------------------------------------------------------

func toAPIRCAAnalysis(db dbgen.RcaAnalysis) RCAAnalysis {
	return RCAAnalysis{
		Id:             db.ID,
		IncidentId:     db.IncidentID,
		Reasoning:      rawJSONToMapVal(db.Reasoning),
		ConclusionCiId: pguuidToUUIDPtr(db.ConclusionAssetID),
		Confidence:     pgnumToFloat32(db.Confidence),
		HumanVerified:  pgboolVal(db.HumanVerified),
	}
}

// ---------------------------------------------------------------------------
// 15. toAPIAdapter
// ---------------------------------------------------------------------------

func toAPIAdapter(db dbgen.IntegrationAdapter) IntegrationAdapter {
	createdAt := pgtsToTime(db.CreatedAt)
	return IntegrationAdapter{
		Id:        (*uuid.UUID)(&db.ID),
		Name:      &db.Name,
		Type:      &db.Type,
		Direction: &db.Direction,
		Endpoint:  pgtextToPtr(db.Endpoint),
		Enabled:   pgboolToPtr(db.Enabled),
		CreatedAt: &createdAt,
	}
}

// ---------------------------------------------------------------------------
// 16. toAPIWebhook
// ---------------------------------------------------------------------------

func toAPIAlertRule(db dbgen.AlertRule) AlertRule {
	return AlertRule{
		Id:         db.ID,
		Name:       db.Name,
		MetricName: db.MetricName,
		Condition:  rawJSONToMapVal(db.Condition),
		Severity:   db.Severity,
		Enabled:    db.Enabled,
		CreatedAt:  db.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// 17. toAPIWebhook
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 18. toAPIIncident
// ---------------------------------------------------------------------------

func toAPIIncident(db dbgen.Incident) Incident {
	return Incident{
		Id:         db.ID,
		Title:      db.Title,
		Status:     db.Status,
		Severity:   db.Severity,
		StartedAt:  db.StartedAt,
		ResolvedAt: pgtsToTimePtr(db.ResolvedAt),
	}
}

func toAPIWebhook(db dbgen.WebhookSubscription) WebhookSubscription {
	createdAt := pgtsToTime(db.CreatedAt)
	return WebhookSubscription{
		Id:        (*uuid.UUID)(&db.ID),
		Name:      &db.Name,
		Url:       &db.Url,
		Events:    &db.Events,
		Enabled:   pgboolToPtr(db.Enabled),
		CreatedAt: &createdAt,
	}
}
