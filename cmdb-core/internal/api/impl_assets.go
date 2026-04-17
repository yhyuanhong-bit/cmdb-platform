package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Asset endpoints
// ---------------------------------------------------------------------------

// ListAssets returns a paginated, filtered list of assets.
// (GET /assets)
func (s *APIServer) ListAssets(c *gin.Context, params ListAssetsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	assets, total, err := s.assetSvc.List(c.Request.Context(), asset.ListParams{
		TenantID:     tenantID,
		Type:         params.Type,
		Status:       params.Status,
		LocationID:   uuidPtrFromOAPI(params.LocationId),
		RackID:       uuidPtrFromOAPI(params.RackId),
		SerialNumber: params.SerialNumber,
		Search:       params.Search,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		response.InternalError(c, "failed to list assets")
		return
	}

	response.OKList(c, convertSlice(assets, toAPIAsset), page, pageSize, int(total))
}

// CreateAsset creates a new asset.
// (POST /assets)
func (s *APIServer) CreateAsset(c *gin.Context) {
	var req CreateAssetJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	attrsJSON := json.RawMessage(`{}`)
	if req.Attributes != nil {
		attrsJSON, _ = json.Marshal(req.Attributes)
	}

	params := dbgen.CreateAssetParams{
		TenantID:               tenantID,
		AssetTag:               req.AssetTag,
		PropertyNumber:         textFromPtr(req.PropertyNumber),
		ControlNumber:          textFromPtr(req.ControlNumber),
		Name:                   req.Name,
		Type:                   req.Type,
		SubType:                pgtype.Text{String: req.SubType, Valid: req.SubType != ""},
		Status:                 req.Status,
		BiaLevel:               req.BiaLevel,
		RackID:                 pguuidFromPtr(uuidPtrFromOAPI(req.RackId)),
		Vendor:                 pgtype.Text{String: req.Vendor, Valid: req.Vendor != ""},
		Model:                  pgtype.Text{String: req.Model, Valid: req.Model != ""},
		SerialNumber:           pgtype.Text{String: req.SerialNumber, Valid: req.SerialNumber != ""},
		Attributes:             attrsJSON,
		Tags:                   req.Tags,
		BmcIp:                  textFromPtr(req.BmcIp),
		BmcType:                textFromPtr(req.BmcType),
		BmcFirmware:            textFromPtr(req.BmcFirmware),
		PurchaseDate:           dateFromPtr(req.PurchaseDate),
		PurchaseCost:           numericFromFloat64Ptr(req.PurchaseCost),
		WarrantyStart:          dateFromPtr(req.WarrantyStart),
		WarrantyEnd:            dateFromPtr(req.WarrantyEnd),
		WarrantyVendor:         textFromPtr(req.WarrantyVendor),
		WarrantyContract:       textFromPtr(req.WarrantyContract),
		ExpectedLifespanMonths: int4FromIntPtr(req.ExpectedLifespanMonths),
		EolDate:                dateFromPtr(req.EolDate),
	}

	// Quality gate: check minimum data quality before creation.
	if s.qualitySvc != nil {
		rackPtr := uuidPtrFromPGUUID(params.RackID)
		qResult, qErr := s.qualitySvc.ValidateForCreation(
			c.Request.Context(), tenantID,
			params.Type, params.Name, params.Status,
			rackPtr,
			params.Vendor.String, params.Model.String, params.SerialNumber.String,
		)
		if qErr != nil {
			c.JSON(422, gin.H{
				"error": gin.H{
					"code":    "QUALITY_GATE_FAILED",
					"message": qErr.Error(),
				},
				"meta": gin.H{
					"quality_score": qResult.Total,
					"issues":        qResult.Issues,
					"request_id":    c.GetString("request_id"),
				},
			})
			return
		}
	}

	created, err := s.assetSvc.Create(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			response.Err(c, 409, "DUPLICATE", "An asset with this asset tag already exists")
			return
		}
		zap.L().Error("failed to create asset", zap.Error(err))
		response.InternalError(c, "failed to create asset")
		return
	}
	s.recordAudit(c, "asset.created", "asset", "asset", created.ID, map[string]any{
		"asset_tag": created.AssetTag,
		"name":      created.Name,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectAssetCreated, tenantID.String(), map[string]any{
		"asset_id": created.ID.String(), "tenant_id": tenantID.String(), "asset_tag": created.AssetTag, "type": created.Type,
	})

	// CIType soft validation: warn about missing recommended attributes.
	warnings := ciTypeSoftValidation(req.Type, req.Attributes)
	if len(warnings) > 0 {
		c.JSON(201, gin.H{
			"data": toAPIAsset(*created),
			"meta": gin.H{"warnings": warnings, "request_id": c.GetString("request_id")},
		})
		return
	}
	response.Created(c, toAPIAsset(*created))
}

// GetAsset returns a single asset by ID.
// (GET /assets/{id})
func (s *APIServer) GetAsset(c *gin.Context, id IdPath) {
	a, err := s.assetSvc.GetByID(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}
	response.OK(c, toAPIAsset(*a))
}

// UpdateAsset updates an existing asset.
// (PUT /assets/{id})
func (s *APIServer) UpdateAsset(c *gin.Context, id IdPath) {
	var req UpdateAssetJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateAssetParams{
		ID: uuid.UUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.BiaLevel != nil {
		params.BiaLevel = pgtype.Text{String: *req.BiaLevel, Valid: true}
	}
	if req.LocationId != nil {
		u := uuid.UUID(*req.LocationId)
		params.LocationID = pgtype.UUID{Bytes: u, Valid: true}
	}
	if req.RackId != nil {
		u := uuid.UUID(*req.RackId)
		params.RackID = pgtype.UUID{Bytes: u, Valid: true}
	}
	if req.Vendor != nil {
		params.Vendor = pgtype.Text{String: *req.Vendor, Valid: true}
	}
	if req.Model != nil {
		params.Model = pgtype.Text{String: *req.Model, Valid: true}
	}
	if req.SerialNumber != nil {
		params.SerialNumber = pgtype.Text{String: *req.SerialNumber, Valid: true}
	}
	if req.Attributes != nil {
		b, _ := json.Marshal(req.Attributes)
		params.Attributes = b
	}
	if req.Tags != nil {
		params.Tags = *req.Tags
	}
	if req.BmcIp != nil {
		params.BmcIp = pgtype.Text{String: *req.BmcIp, Valid: true}
	}
	if req.BmcType != nil {
		params.BmcType = pgtype.Text{String: *req.BmcType, Valid: true}
	}
	if req.BmcFirmware != nil {
		params.BmcFirmware = pgtype.Text{String: *req.BmcFirmware, Valid: true}
	}
	if req.PurchaseDate != nil {
		params.PurchaseDate = dateFromPtr(req.PurchaseDate)
	}
	if req.PurchaseCost != nil {
		params.PurchaseCost = numericFromFloat64Ptr(req.PurchaseCost)
	}
	if req.WarrantyStart != nil {
		params.WarrantyStart = dateFromPtr(req.WarrantyStart)
	}
	if req.WarrantyEnd != nil {
		params.WarrantyEnd = dateFromPtr(req.WarrantyEnd)
	}
	if req.WarrantyVendor != nil {
		params.WarrantyVendor = pgtype.Text{String: *req.WarrantyVendor, Valid: true}
	}
	if req.WarrantyContract != nil {
		params.WarrantyContract = pgtype.Text{String: *req.WarrantyContract, Valid: true}
	}
	if req.ExpectedLifespanMonths != nil {
		params.ExpectedLifespanMonths = int4FromIntPtr(req.ExpectedLifespanMonths)
	}
	if req.EolDate != nil {
		params.EolDate = dateFromPtr(req.EolDate)
	}

	// Field-level authority check: prevent low-priority API source from
	// overwriting fields owned by higher-priority sources (e.g. IPMI, SNMP).
	const apiSourcePriority = 50
	var authorityWarnings []string

	tenantID := tenantIDFromContext(c)
	authRows, authErr := s.pool.Query(c.Request.Context(),
		`SELECT field_name, MAX(priority) as max_priority
		 FROM asset_field_authorities
		 WHERE tenant_id = $1
		 GROUP BY field_name
		 HAVING MAX(priority) > $2`,
		tenantID, apiSourcePriority)
	if authErr == nil {
		defer authRows.Close()
		blockedFields := make(map[string]int)
		for authRows.Next() {
			var fieldName string
			var maxPriority int
			if authRows.Scan(&fieldName, &maxPriority) == nil {
				blockedFields[fieldName] = maxPriority
			}
		}

		if params.SerialNumber.Valid && blockedFields["serial_number"] > 0 {
			authorityWarnings = append(authorityWarnings, fmt.Sprintf("serial_number is managed by a higher-priority source (priority %d)", blockedFields["serial_number"]))
			params.SerialNumber = pgtype.Text{}
		}
		if params.Vendor.Valid && blockedFields["vendor"] > 0 {
			authorityWarnings = append(authorityWarnings, fmt.Sprintf("vendor is managed by a higher-priority source (priority %d)", blockedFields["vendor"]))
			params.Vendor = pgtype.Text{}
		}
		if params.Model.Valid && blockedFields["model"] > 0 {
			authorityWarnings = append(authorityWarnings, fmt.Sprintf("model is managed by a higher-priority source (priority %d)", blockedFields["model"]))
			params.Model = pgtype.Text{}
		}
	}

	updated, err := s.assetSvc.Update(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "asset not found")
		} else {
			response.InternalError(c, "failed to update asset")
		}
		return
	}

	// Supplementary update for ip_address (not in sqlc-generated query)
	if req.IpAddress != nil {
		s.pool.Exec(c.Request.Context(),
			"UPDATE assets SET ip_address = $1 WHERE id = $2",
			*req.IpAddress, uuid.UUID(id),
		)
	}

	diff := map[string]any{}
	if req.Name != nil {
		diff["name"] = *req.Name
	}
	if req.Status != nil {
		diff["status"] = *req.Status
	}
	if req.BiaLevel != nil {
		diff["bia_level"] = *req.BiaLevel
	}
	if req.Vendor != nil {
		diff["vendor"] = *req.Vendor
	}
	if req.Model != nil {
		diff["model"] = *req.Model
	}
	if req.SerialNumber != nil {
		diff["serial_number"] = *req.SerialNumber
	}
	if req.IpAddress != nil {
		diff["ip_address"] = *req.IpAddress
	}
	s.recordAudit(c, "asset.updated", "asset", "asset", updated.ID, diff)
	s.publishEvent(c.Request.Context(), eventbus.SubjectAssetUpdated, tenantIDFromContext(c).String(), map[string]any{
		"asset_id": updated.ID.String(), "tenant_id": tenantIDFromContext(c).String(),
	})

	// ITSM Change Audit: Critical assets auto-create change audit work order
	var changeOrderID *uuid.UUID
	if updated.BiaLevel == "critical" {
		userID := userIDFromContext(c)
		order, err := s.maintenanceSvc.Create(c.Request.Context(), tenantID, userID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Change Audit: %s (%s)", updated.Name, updated.AssetTag),
			Type:        "change_audit",
			Description: "Critical asset modified. Review required.",
			Priority:    "high",
		})
		if err == nil {
			id := order.ID
			changeOrderID = &id
		}
	}

	// Build meta with optional warnings and change_order_id
	meta := gin.H{"request_id": c.GetString("request_id")}
	if len(authorityWarnings) > 0 {
		meta["warnings"] = authorityWarnings
	}
	if changeOrderID != nil {
		meta["change_order_id"] = changeOrderID.String()
	}

	// Return with meta if it contains more than just request_id
	if len(authorityWarnings) > 0 || changeOrderID != nil {
		c.JSON(200, gin.H{
			"data": toAPIAsset(*updated),
			"meta": meta,
		})
		return
	}
	response.OK(c, toAPIAsset(*updated))
}

// DeleteAsset deletes an asset.
// (DELETE /assets/{id})
func (s *APIServer) DeleteAsset(c *gin.Context, id IdPath) {
	err := s.assetSvc.Delete(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}
	s.recordAudit(c, "asset.deleted", "asset", "asset", uuid.UUID(id), nil)
	s.publishEvent(c.Request.Context(), eventbus.SubjectAssetDeleted, tenantIDFromContext(c).String(), map[string]any{
		"asset_id": uuid.UUID(id).String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	c.Status(204)
}
