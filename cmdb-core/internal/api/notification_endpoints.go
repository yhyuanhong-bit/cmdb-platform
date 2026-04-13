package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// ListNotifications returns unread notifications for the current user.
func (s *APIServer) ListNotifications(c *gin.Context) {
	userID := userIDFromContext(c)
	tenantID := tenantIDFromContext(c)
	rows, err := s.pool.Query(c.Request.Context(),
		`SELECT id, tenant_id, user_id, type, title, body, resource_type, resource_id, is_read, created_at
		 FROM notifications
		 WHERE user_id = $1 AND tenant_id = $2 AND is_read = false
		 ORDER BY created_at DESC
		 LIMIT 50`,
		userID, tenantID)
	if err != nil {
		response.InternalError(c, "failed to list notifications")
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var (
			id           uuid.UUID
			tenantID     uuid.UUID
			uid          uuid.UUID
			nType        string
			title        string
			body         *string
			resourceType *string
			resourceID   *uuid.UUID
			isRead       bool
			createdAt    interface{}
		)
		if err := rows.Scan(&id, &tenantID, &uid, &nType, &title, &body, &resourceType, &resourceID, &isRead, &createdAt); err != nil {
			continue
		}
		item := gin.H{
			"id":            id.String(),
			"type":          nType,
			"title":         title,
			"body":          body,
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"is_read":       isRead,
			"created_at":    createdAt,
		}
		items = append(items, item)
	}
	if items == nil {
		items = []gin.H{}
	}
	response.OK(c, items)
}

// CountUnreadNotifications returns count of unread notifications.
func (s *APIServer) CountUnreadNotifications(c *gin.Context) {
	userID := userIDFromContext(c)
	tenantID := tenantIDFromContext(c)
	var count int64
	_ = s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM notifications WHERE user_id = $1 AND tenant_id = $2 AND is_read = false",
		userID, tenantID).Scan(&count)
	response.OK(c, gin.H{"count": count})
}

// MarkNotificationRead marks a single notification as read.
func (s *APIServer) MarkNotificationRead(c *gin.Context) {
	userID := userIDFromContext(c)
	tenantID := tenantIDFromContext(c)
	notifID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid notification ID")
		return
	}
	_, _ = s.pool.Exec(c.Request.Context(),
		"UPDATE notifications SET is_read = true WHERE id = $1 AND user_id = $2 AND tenant_id = $3",
		notifID, userID, tenantID)
	c.Status(204)
}

// MarkAllNotificationsRead marks all notifications as read for the current user.
func (s *APIServer) MarkAllNotificationsRead(c *gin.Context) {
	userID := userIDFromContext(c)
	tenantID := tenantIDFromContext(c)
	_, _ = s.pool.Exec(c.Request.Context(),
		"UPDATE notifications SET is_read = true WHERE user_id = $1 AND tenant_id = $2 AND is_read = false",
		userID, tenantID)
	c.Status(204)
}
