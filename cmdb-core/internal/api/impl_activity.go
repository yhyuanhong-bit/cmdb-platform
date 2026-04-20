package api

import (
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetActivityFeed handles GET /activity-feed?target_type=rack&target_id=uuid
// Returns a unified activity feed (audit events, alert events, work order logs)
// for a given target, ordered by timestamp descending.
func (s *APIServer) GetActivityFeed(c *gin.Context, params GetActivityFeedParams) {
	tenantID := tenantIDFromContext(c)
	targetType := string(params.TargetType)
	targetID := uuid.UUID(params.TargetId).String()

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT event_type, action, description, timestamp, severity, operator
		FROM (
			-- Arm 1: audit events
			SELECT
				'audit'                      AS event_type,
				ae.action                    AS action,
				COALESCE(ae.module, '')      AS description,
				ae.created_at                AS timestamp,
				''                           AS severity,
				COALESCE(u.display_name, '') AS operator
			FROM audit_events ae
			LEFT JOIN users u ON ae.operator_id = u.id
			WHERE ae.tenant_id = $3
			  AND (
				  ($1 = 'rack'     AND ae.target_type = 'rack'     AND ae.target_id::text = $2)
				  OR ($1 = 'asset'    AND ae.target_type = 'asset'    AND ae.target_id::text = $2)
				  OR ($1 = 'location' AND ae.target_type = 'location' AND ae.target_id::text = $2)
			  )

			UNION ALL

			-- Arm 2: alert events
			SELECT
				'alert'                                    AS event_type,
				COALESCE(ale.status, '')                   AS action,
				COALESCE(ale.message, '')                  AS description,
				COALESCE(ale.fired_at, now())              AS timestamp,
				COALESCE(ale.severity, '')                 AS severity,
				''                                         AS operator
			FROM alert_events ale
			JOIN assets a ON ale.asset_id = a.id
			WHERE ale.tenant_id = $3
			  AND (
				  ($1 = 'asset'    AND ale.asset_id::text = $2)
				  OR ($1 = 'rack'  AND a.rack_id::text   = $2)
				  OR ($1 = 'location' AND a.location_id::text = $2)
			  )

			UNION ALL

			-- Arm 3: work order logs
			SELECT
				'work_order'                               AS event_type,
				COALESCE(wol.action, wo.status, '')        AS action,
				COALESCE(wol.comment, wo.title, '')        AS description,
				COALESCE(wol.created_at, wo.created_at)    AS timestamp,
				''                                         AS severity,
				COALESCE(u2.display_name, '')              AS operator
			FROM work_order_logs wol
			JOIN work_orders wo ON wol.order_id = wo.id
			JOIN assets a2      ON wo.asset_id = a2.id
			LEFT JOIN users u2  ON wol.operator_id = u2.id
			WHERE wo.tenant_id = $3
			  AND (
				  ($1 = 'asset'    AND wo.asset_id::text   = $2)
				  OR ($1 = 'rack'  AND a2.rack_id::text    = $2)
				  OR ($1 = 'location' AND a2.location_id::text = $2)
			  )
		) combined
		ORDER BY timestamp DESC
		LIMIT 20
	`, targetType, targetID, tenantID)
	if err != nil {
		response.InternalError(c, "failed to query activity feed")
		return
	}
	defer rows.Close()

	events := []gin.H{}
	for rows.Next() {
		var eventType, action, description, severity, operator string
		var timestamp time.Time
		if err := rows.Scan(&eventType, &action, &description, &timestamp, &severity, &operator); err != nil {
			continue
		}
		events = append(events, gin.H{
			"event_type":  eventType,
			"action":      action,
			"description": description,
			"timestamp":   timestamp.Format(time.RFC3339),
			"severity":    severity,
			"operator":    operator,
		})
	}

	response.OK(c, gin.H{"events": events})
}

// GetAuditEventDetail handles GET /audit/events/:id
// Returns full detail of a single audit event including operator info and diff.
func (s *APIServer) GetAuditEventDetail(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	eventID := uuid.UUID(id)

	row := s.pool.QueryRow(c.Request.Context(), `
		SELECT
			ae.id,
			ae.action,
			COALESCE(ae.module, '')       AS module,
			COALESCE(ae.target_type, '')  AS target_type,
			ae.target_id,
			ae.operator_id,
			COALESCE(ae.diff, '{}')       AS diff,
			COALESCE(ae.source, '')       AS source,
			ae.created_at,
			COALESCE(u.display_name, '') AS display_name,
			COALESCE(u.email, '')        AS email
		FROM audit_events ae
		LEFT JOIN users u ON ae.operator_id = u.id
		WHERE ae.id = $1 AND ae.tenant_id = $2
	`, eventID, tenantID)

	var eventIDStr, action, module, targetType, source, displayName, email string
	var diff []byte
	var createdAt time.Time
	var targetID pgtype.UUID
	var operatorID pgtype.UUID

	if err := row.Scan(
		&eventIDStr, &action, &module, &targetType,
		&targetID, &operatorID,
		&diff, &source, &createdAt,
		&displayName, &email,
	); err != nil {
		response.NotFound(c, "audit event not found")
		return
	}

	// Format nullable UUIDs
	var targetIDStr *string
	if targetID.Valid {
		s := uuid.UUID(targetID.Bytes).String()
		targetIDStr = &s
	}
	var operatorIDStr *string
	if operatorID.Valid {
		s := uuid.UUID(operatorID.Bytes).String()
		operatorIDStr = &s
	}

	response.OK(c, gin.H{
		"event": gin.H{
			"id":             eventIDStr,
			"action":         action,
			"module":         module,
			"target_type":    targetType,
			"target_id":      targetIDStr,
			"operator_id":    operatorIDStr,
			"operator_name":  displayName,
			"operator_email": email,
			"diff":           string(diff),
			"source":         source,
			"created_at":     createdAt.Format(time.RFC3339),
		},
	})
}
