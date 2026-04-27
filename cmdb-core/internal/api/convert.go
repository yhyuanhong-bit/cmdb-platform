package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
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

// toAPIAssetSnapshot converts a dbgen.AssetSnapshot into the API type.
// Separate from toAPIAsset because the snapshot omits warranty/lifecycle
// fields the point-in-time view does not capture — the shape is
// intentionally leaner than the live asset DTO.
func toAPIAssetSnapshot(db dbgen.AssetSnapshot) AssetSnapshot {
	var tags *[]string
	if db.Tags != nil {
		t := []string(db.Tags)
		tags = &t
	}
	return AssetSnapshot{
		Id:           db.ID,
		AssetId:      db.AssetID,
		TenantId:     db.TenantID,
		ValidAt:      db.ValidAt,
		Name:         db.Name,
		AssetTag:     db.AssetTag,
		Status:       db.Status,
		BiaLevel:     db.BiaLevel,
		LocationId:   pguuidToUUIDPtr(db.LocationID),
		RackId:       pguuidToUUIDPtr(db.RackID),
		Vendor:       pgtextToPtr(db.Vendor),
		Model:        pgtextToPtr(db.Model),
		SerialNumber: pgtextToPtr(db.SerialNumber),
		Attributes:   rawJSONToMapVal(db.Attributes),
		Tags:         tags,
		OwnerTeam:    pgtextToPtr(db.OwnerTeam),
	}
}

// ---------------------------------------------------------------------------
// 1. toAPIAsset
// ---------------------------------------------------------------------------

func toAPIAsset(db dbgen.Asset) Asset {
	return Asset{
		Id:                     db.ID,
		AssetTag:               db.AssetTag,
		PropertyNumber:         pgtextToPtr(db.PropertyNumber),
		ControlNumber:          pgtextToPtr(db.ControlNumber),
		Name:                   db.Name,
		Type:                   db.Type,
		SubType:                pgtextToStr(db.SubType),
		Status:                 db.Status,
		BiaLevel:               db.BiaLevel,
		LocationId:             pguuidToUUIDPtr(db.LocationID),
		RackId:                 pguuidToUUIDPtr(db.RackID),
		Vendor:                 pgtextToStr(db.Vendor),
		Model:                  pgtextToStr(db.Model),
		SerialNumber:           pgtextToStr(db.SerialNumber),
		Attributes:             rawJSONToMapVal(db.Attributes),
		Tags:                   db.Tags,
		BmcIp:                  pgtextToPtr(db.BmcIp),
		BmcType:                pgtextToPtr(db.BmcType),
		BmcFirmware:            pgtextToPtr(db.BmcFirmware),
		PurchaseDate:           pgdateToPtr(db.PurchaseDate),
		PurchaseCost:           pgnumToPtr(db.PurchaseCost),
		WarrantyStart:          pgdateToPtr(db.WarrantyStart),
		WarrantyEnd:            pgdateToPtr(db.WarrantyEnd),
		WarrantyVendor:         pgtextToPtr(db.WarrantyVendor),
		WarrantyContract:       pgtextToPtr(db.WarrantyContract),
		ExpectedLifespanMonths: pgint4ToIntPtr(db.ExpectedLifespanMonths),
		EolDate:                pgdateToPtr(db.EolDate),
		OwnerTeam:              pgtextToPtr(db.OwnerTeam),
		CreatedAt:              db.CreatedAt,
		UpdatedAt:              db.UpdatedAt,
	}
}

func pgint4ToIntPtr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
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

func pgFloat8ToPtr(v pgtype.Float8) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

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
		Latitude:  pgFloat8ToPtr(db.Latitude),
		Longitude: pgFloat8ToPtr(db.Longitude),
	}
}

// ---------------------------------------------------------------------------
// 3. toAPIRack
// ---------------------------------------------------------------------------

func toAPIRack(db dbgen.Rack) Rack {
	return toAPIRackWithOccupancy(db, 0)
}

// toAPIRackWithOccupancy converts a dbgen.Rack to API Rack with computed used_u.
func toAPIRackWithOccupancy(db dbgen.Rack, usedU int) Rack {
	return Rack{
		Id:              db.ID,
		LocationId:      db.LocationID,
		Name:            db.Name,
		RowLabel:        pgtextToStr(db.RowLabel),
		TotalU:          int(db.TotalU),
		UsedU:           usedU,
		PowerCapacityKw: pgnumToFloat32(db.PowerCapacityKw),
		Status:          db.Status,
		Tags:            db.Tags,
		CreatedAt:       db.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// 4. toAPIWorkOrder
// ---------------------------------------------------------------------------

func toAPIWorkOrder(db dbgen.WorkOrder) gin.H {
	return gin.H{
		"id":               db.ID,
		"code":             db.Code,
		"title":            db.Title,
		"type":             db.Type,
		"status":           db.Status,
		"priority":         db.Priority,
		"location_id":      pgUUIDToUUID(db.LocationID),
		"requestor_id":     pguuidToUUIDPtr(db.RequestorID),
		"assignee_id":      pguuidToUUIDPtr(db.AssigneeID),
		"description":      pgtextToStr(db.Description),
		"reason":           pgtextToStr(db.Reason),
		"scheduled_start":  pgtsToTime(db.ScheduledStart),
		"scheduled_end":    pgtsToTime(db.ScheduledEnd),
		"actual_start":     pgtsToTimePtr(db.ActualStart),
		"actual_end":       pgtsToTimePtr(db.ActualEnd),
		"created_at":       db.CreatedAt,
		"approved_at":      pgtsToTimePtr(db.ApprovedAt),
		"approved_by":      pguuidToUUIDPtr(db.ApprovedBy),
		"sla_deadline":     pgtsToTimePtr(db.SlaDeadline),
		"sla_warning_sent": db.SlaWarningSent,
		"sla_breached":     db.SlaBreached,
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
		Enabled:  db.Enabled,
	}
}

// ---------------------------------------------------------------------------
// 13. toAPIRCAAnalysis
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
	inc := Incident{
		Id:         db.ID,
		Title:      db.Title,
		Status:     IncidentStatus(db.Status),
		Severity:   db.Severity,
		StartedAt:  db.StartedAt,
		ResolvedAt: pgtsToTimePtr(db.ResolvedAt),
	}
	if db.Priority.Valid {
		p := IncidentPriority(db.Priority.String)
		inc.Priority = &p
	}
	if db.Description.Valid {
		s := db.Description.String
		inc.Description = &s
	}
	if db.Impact.Valid {
		s := db.Impact.String
		inc.Impact = &s
	}
	if db.RootCause.Valid {
		s := db.RootCause.String
		inc.RootCause = &s
	}
	if db.AssigneeUserID.Valid {
		u := uuid.UUID(db.AssigneeUserID.Bytes)
		inc.AssigneeUserId = &u
	}
	if db.AffectedAssetID.Valid {
		u := uuid.UUID(db.AffectedAssetID.Bytes)
		inc.AffectedAssetId = &u
	}
	if db.AffectedServiceID.Valid {
		u := uuid.UUID(db.AffectedServiceID.Bytes)
		inc.AffectedServiceId = &u
	}
	if db.AcknowledgedAt.Valid {
		t := db.AcknowledgedAt.Time
		inc.AcknowledgedAt = &t
	}
	if db.AcknowledgedBy.Valid {
		u := uuid.UUID(db.AcknowledgedBy.Bytes)
		inc.AcknowledgedBy = &u
	}
	if db.ResolvedBy.Valid {
		u := uuid.UUID(db.ResolvedBy.Bytes)
		inc.ResolvedBy = &u
	}
	if !db.UpdatedAt.IsZero() {
		t := db.UpdatedAt
		inc.UpdatedAt = &t
	}
	return inc
}

// toAPIIncidentComment converts a list-row (has author_username denormalised)
// to the API shape.
func toAPIIncidentComment(db dbgen.ListIncidentCommentsRow) IncidentComment {
	out := IncidentComment{
		Id:         db.ID,
		IncidentId: db.IncidentID,
		Kind:       IncidentCommentKind(db.Kind),
		Body:       db.Body,
		CreatedAt:  db.CreatedAt,
	}
	if db.AuthorID.Valid {
		u := uuid.UUID(db.AuthorID.Bytes)
		out.AuthorId = &u
	}
	if db.AuthorUsername.Valid {
		s := db.AuthorUsername.String
		out.AuthorUsername = &s
	}
	return out
}


// toAPIIncidentCommentFromRecord is the create-path variant that doesn't
// have author_username joined in. The UI re-fetches the list after a POST
// anyway, so leaving username empty on the 201 response is fine.
func toAPIIncidentCommentFromRecord(db dbgen.IncidentComment) IncidentComment {
	out := IncidentComment{
		Id:         db.ID,
		IncidentId: db.IncidentID,
		Kind:       IncidentCommentKind(db.Kind),
		Body:       db.Body,
		CreatedAt:  db.CreatedAt,
	}
	if db.AuthorID.Valid {
		u := uuid.UUID(db.AuthorID.Bytes)
		out.AuthorId = &u
	}
	return out
}

func toAPIWebhookDelivery(db dbgen.WebhookDelivery) WebhookDelivery {
	id := (*uuid.UUID)(&db.ID)
	subID := (*uuid.UUID)(&db.SubscriptionID)
	eventType := db.EventType
	payload := rawJSONToMap(db.Payload)
	var statusCode *int
	if db.StatusCode.Valid {
		v := int(db.StatusCode.Int32)
		statusCode = &v
	}
	responseBody := pgtextToPtr(db.ResponseBody)
	deliveredAt := pgtsToTimePtr(db.DeliveredAt)
	return WebhookDelivery{
		Id:             id,
		SubscriptionId: subID,
		EventType:      &eventType,
		Payload:        payload,
		StatusCode:     statusCode,
		ResponseBody:   responseBody,
		DeliveredAt:    deliveredAt,
	}
}

// ---------------------------------------------------------------------------
// 19. toAPIBIAAssessment
// ---------------------------------------------------------------------------

func toAPIBIAAssessment(db dbgen.BiaAssessment) BIAAssessment {
	a := BIAAssessment{
		Id:              db.ID,
		SystemName:      db.SystemName,
		SystemCode:      db.SystemCode,
		Owner:           pgtextToPtr(db.Owner),
		BiaScore:        int(db.BiaScore),
		Tier:            db.Tier,
		RtoHours:        pgnumToFloat32Ptr(db.RtoHours),
		RpoMinutes:      pgnumToFloat32Ptr(db.RpoMinutes),
		MtpdHours:       pgnumToFloat32Ptr(db.MtpdHours),
		DataCompliance:  pgboolToPtr(db.DataCompliance),
		AssetCompliance: pgboolToPtr(db.AssetCompliance),
		AuditCompliance: pgboolToPtr(db.AuditCompliance),
		Description:     pgtextToPtr(db.Description),
		LastAssessed:    pgtsToTimePtr(db.LastAssessed),
		AssessedBy:      pguuidToUUIDPtr(db.AssessedBy),
		CreatedAt:       &db.CreatedAt,
	}
	return a
}

// ---------------------------------------------------------------------------
// 20. toAPIBIAScoringRule
// ---------------------------------------------------------------------------

func toAPIBIAScoringRule(db dbgen.BiaScoringRule) BIAScoringRule {
	return BIAScoringRule{
		Id:           db.ID,
		TierName:     db.TierName,
		TierLevel:    int(db.TierLevel),
		DisplayName:  db.DisplayName,
		MinScore:     int(db.MinScore),
		MaxScore:     int(db.MaxScore),
		RtoThreshold: pgnumToFloat32Ptr(db.RtoThreshold),
		RpoThreshold: pgnumToFloat32Ptr(db.RpoThreshold),
		Description:  pgtextToPtr(db.Description),
		Color:        pgtextToPtr(db.Color),
		Icon:         pgtextToPtr(db.Icon),
	}
}

// ---------------------------------------------------------------------------
// 21. toAPIBIADependency
// ---------------------------------------------------------------------------

func toAPIBIADependency(db dbgen.BiaDependency) BIADependency {
	return BIADependency{
		Id:             db.ID,
		AssessmentId:   db.AssessmentID,
		AssetId:        db.AssetID,
		DependencyType: db.DependencyType,
		Criticality:    pgtextToPtr(db.Criticality),
	}
}

// pgnumToFloat32Ptr converts a pgtype.Numeric to *float32.
func pgnumToFloat32Ptr(v pgtype.Numeric) *float32 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	val := float32(f.Float64)
	return &val
}

// ---------------------------------------------------------------------------
// 22. toAPIRackSlot
// ---------------------------------------------------------------------------

func toAPIRackSlot(db dbgen.ListRackSlotsRow) RackSlot {
	id := uuid.UUID(db.ID)
	rackID := uuid.UUID(db.RackID)
	assetID := uuid.UUID(db.AssetID)
	startU := int(db.StartU)
	endU := int(db.EndU)
	return RackSlot{
		Id:        &id,
		RackId:    &rackID,
		AssetId:   &assetID,
		StartU:    &startU,
		EndU:      &endU,
		Side:      &db.Side,
		AssetName: pgtextToPtr(db.AssetName),
		AssetTag:  pgtextToPtr(db.AssetTag),
		AssetType: pgtextToPtr(db.AssetType),
		BiaLevel:  pgtextToPtr(db.BiaLevel),
	}
}

// ---------------------------------------------------------------------------
// 23a. toAPIQualityFlag
// ---------------------------------------------------------------------------

func toAPIQualityFlag(db dbgen.QualityFlag) QualityFlag {
	f := QualityFlag{
		Id:           db.ID,
		TenantId:     db.TenantID,
		AssetId:      db.AssetID,
		ReporterType: QualityFlagReporterType(db.ReporterType),
		Severity:     QualityFlagSeverity(db.Severity),
		Category:     db.Category,
		Message:      db.Message,
		Status:       QualityFlagStatus(db.Status),
		CreatedAt:    db.CreatedAt,
	}
	if db.ReporterID.Valid {
		rid := uuid.UUID(db.ReporterID.Bytes)
		f.ReporterId = &rid
	}
	if db.ResolvedAt.Valid {
		ra := db.ResolvedAt.Time
		f.ResolvedAt = &ra
	}
	if db.ResolvedBy.Valid {
		rb := uuid.UUID(db.ResolvedBy.Bytes)
		f.ResolvedBy = &rb
	}
	if db.ResolutionNote.Valid {
		rn := db.ResolutionNote.String
		f.ResolutionNote = &rn
	}
	return f
}

func toAPIQualityFlagListItem(db dbgen.ListOpenQualityFlagsRow) QualityFlagListItem {
	item := QualityFlagListItem{
		Id:           db.ID,
		TenantId:     db.TenantID,
		AssetId:      db.AssetID,
		ReporterType: QualityFlagListItemReporterType(db.ReporterType),
		Severity:     QualityFlagListItemSeverity(db.Severity),
		Category:     db.Category,
		Message:      db.Message,
		Status:       QualityFlagListItemStatus(db.Status),
		CreatedAt:    db.CreatedAt,
		AssetName:    &db.AssetName,
		AssetTag:     &db.AssetTag,
	}
	if db.ReporterID.Valid {
		rid := uuid.UUID(db.ReporterID.Bytes)
		item.ReporterId = &rid
	}
	if db.ResolvedAt.Valid {
		ra := db.ResolvedAt.Time
		item.ResolvedAt = &ra
	}
	if db.ResolvedBy.Valid {
		rb := uuid.UUID(db.ResolvedBy.Bytes)
		item.ResolvedBy = &rb
	}
	if db.ResolutionNote.Valid {
		rn := db.ResolutionNote.String
		item.ResolutionNote = &rn
	}
	return item
}

// ---------------------------------------------------------------------------
// 23. toAPIQualityRule
// ---------------------------------------------------------------------------

func toAPIQualityRule(db dbgen.QualityRule) QualityRule {
	r := QualityRule{
		Id:        db.ID,
		Dimension: db.Dimension,
		FieldName: db.FieldName,
		RuleType:  db.RuleType,
		CreatedAt: &db.CreatedAt,
	}
	if db.CiType.Valid {
		r.CiType = &db.CiType.String
	}
	if db.Weight.Valid {
		w := int(db.Weight.Int32)
		r.Weight = &w
	}
	if db.Enabled.Valid {
		r.Enabled = &db.Enabled.Bool
	}
	r.RuleConfig = bytesToJSON(db.RuleConfig)
	return r
}

// ---------------------------------------------------------------------------
// 24. toAPIQualityScoreFromWorst
// ---------------------------------------------------------------------------

func toAPIQualityScoreFromWorst(db dbgen.GetWorstAssetsRow) QualityScore {
	id := db.ID
	assetID := db.AssetID
	completeness := pgnumToFloat32(db.Completeness)
	accuracy := pgnumToFloat32(db.Accuracy)
	timeliness := pgnumToFloat32(db.Timeliness)
	consistency := pgnumToFloat32(db.Consistency)
	totalScore := pgnumToFloat32(db.TotalScore)
	return QualityScore{
		Id:           &id,
		AssetId:      &assetID,
		Completeness: &completeness,
		Accuracy:     &accuracy,
		Timeliness:   &timeliness,
		Consistency:  &consistency,
		TotalScore:   &totalScore,
		IssueDetails: bytesToIssueDetails(db.IssueDetails),
		ScanDate:     &db.ScanDate,
		AssetName:    &db.AssetName,
		AssetTag:     &db.AssetTag,
	}
}

// ---------------------------------------------------------------------------
// 25. toAPIQualityScoreFromHistory
// ---------------------------------------------------------------------------

func toAPIQualityScoreFromHistory(db dbgen.QualityScore) QualityScore {
	id := db.ID
	assetID := db.AssetID
	completeness := pgnumToFloat32(db.Completeness)
	accuracy := pgnumToFloat32(db.Accuracy)
	timeliness := pgnumToFloat32(db.Timeliness)
	consistency := pgnumToFloat32(db.Consistency)
	totalScore := pgnumToFloat32(db.TotalScore)
	return QualityScore{
		Id:           &id,
		AssetId:      &assetID,
		Completeness: &completeness,
		Accuracy:     &accuracy,
		Timeliness:   &timeliness,
		Consistency:  &consistency,
		TotalScore:   &totalScore,
		IssueDetails: bytesToIssueDetails(db.IssueDetails),
		ScanDate:     &db.ScanDate,
	}
}

// ---------------------------------------------------------------------------
// 26. toAPIQualityDashboard
// ---------------------------------------------------------------------------

func toAPIQualityDashboard(db dbgen.GetQualityDashboardRow) QualityDashboard {
	avgTotal := interfaceToFloat32(db.AvgTotal)
	avgCompleteness := interfaceToFloat32(db.AvgCompleteness)
	avgAccuracy := interfaceToFloat32(db.AvgAccuracy)
	avgTimeliness := interfaceToFloat32(db.AvgTimeliness)
	avgConsistency := interfaceToFloat32(db.AvgConsistency)
	totalScanned := int(db.TotalScanned)
	return QualityDashboard{
		AvgTotal:        &avgTotal,
		AvgCompleteness: &avgCompleteness,
		AvgAccuracy:     &avgAccuracy,
		AvgTimeliness:   &avgTimeliness,
		AvgConsistency:  &avgConsistency,
		TotalScanned:    &totalScanned,
	}
}

func bytesToIssueDetails(b []byte) *[]map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	var issues []map[string]interface{}
	if err := json.Unmarshal(b, &issues); err != nil {
		return nil
	}
	return &issues
}

func interfaceToFloat32(v interface{}) float32 {
	switch val := v.(type) {
	case float64:
		return float32(val)
	case float32:
		return val
	case int64:
		return float32(val)
	case string:
		// pgtype.Numeric may serialize as string
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return float32(f)
	default:
		// Try numeric decode via pgtype
		if n, ok := v.(pgtype.Numeric); ok {
			return pgnumToFloat32(n)
		}
		return 0
	}
}

// ---------------------------------------------------------------------------
// 27. toAPIDiscoveredAsset
// ---------------------------------------------------------------------------

func toAPIDiscoveredAsset(db dbgen.DiscoveredAsset) DiscoveredAsset {
	src := db.Source
	status := DiscoveredAssetStatus(db.Status)
	hostname := pgtextToPtr(db.Hostname)
	externalID := pgtextToPtr(db.ExternalID)
	ipAddr := pgtextToPtr(db.IpAddress)
	rawData := rawJSONToMap(db.RawData)
	matchedAssetID := pguuidToOAPIUUIDPtr(db.MatchedAssetID)
	approvedAssetID := pguuidToOAPIUUIDPtr(db.ApprovedAssetID)
	diffDetails := bytesToJSON(db.DiffDetails)
	discoveredAt := db.DiscoveredAt
	reviewedBy := pguuidToOAPIUUIDPtr(db.ReviewedBy)
	var reviewedAt *time.Time
	if db.ReviewedAt.Valid {
		t := db.ReviewedAt.Time
		reviewedAt = &t
	}
	// Wave 3 fields: surface match_confidence / match_strategy /
	// review_reason so the review UI can render trust signals and the
	// audit trail captures every approve/ignore decision.
	var matchConf *float32
	if db.MatchConfidence.Valid {
		f := pgnumToFloat32(db.MatchConfidence)
		matchConf = &f
	}
	var matchStrat *DiscoveredAssetMatchStrategy
	if db.MatchStrategy.Valid {
		ms := DiscoveredAssetMatchStrategy(db.MatchStrategy.String)
		matchStrat = &ms
	}
	var reviewReason *string
	if db.ReviewReason.Valid {
		v := db.ReviewReason.String
		reviewReason = &v
	}

	id := openapi_types.UUID(db.ID)
	return DiscoveredAsset{
		Id:              &id,
		Source:          &src,
		ExternalId:      externalID,
		Hostname:        hostname,
		IpAddress:       ipAddr,
		RawData:         rawData,
		Status:          &status,
		MatchedAssetId:  matchedAssetID,
		ApprovedAssetId: approvedAssetID,
		DiffDetails:     diffDetails,
		DiscoveredAt:    &discoveredAt,
		ReviewedBy:      reviewedBy,
		ReviewedAt:      reviewedAt,
		MatchConfidence: matchConf,
		MatchStrategy:   matchStrat,
		ReviewReason:    reviewReason,
	}
}

// pguuidToOAPIUUIDPtr converts pgtype.UUID to *openapi_types.UUID.
func pguuidToOAPIUUIDPtr(v pgtype.UUID) *openapi_types.UUID {
	if !v.Valid {
		return nil
	}
	u := openapi_types.UUID(v.Bytes)
	return &u
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
