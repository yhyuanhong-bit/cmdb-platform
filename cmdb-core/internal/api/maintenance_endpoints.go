package api

import (
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListWorkOrderComments handles GET /maintenance/orders/{id}/comments
// Returns all comments for a specific work order, ordered oldest-first.
func (s *APIServer) ListWorkOrderComments(c *gin.Context, id IdPath) {
	orderID := uuid.UUID(id)

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT wc.id, u.display_name, wc.text, wc.created_at
		FROM work_order_comments wc
		LEFT JOIN users u ON wc.author_id = u.id
		WHERE wc.order_id = $1
		ORDER BY wc.created_at ASC
	`, orderID)
	if err != nil {
		response.InternalError(c, "failed to query comments")
		return
	}
	defer rows.Close()

	comments := []gin.H{}
	for rows.Next() {
		var (
			id         uuid.UUID
			authorName *string
			text       string
			createdAt  time.Time
		)
		if err := rows.Scan(&id, &authorName, &text, &createdAt); err != nil {
			response.InternalError(c, "failed to scan row")
			return
		}
		comments = append(comments, gin.H{
			"id":          id.String(),
			"author_name": authorName,
			"text":        text,
			"created_at":  createdAt.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, "error reading comment rows")
		return
	}

	response.OK(c, gin.H{"comments": comments})
}

// CreateWorkOrderComment handles POST /maintenance/orders/{id}/comments
// Creates a new comment on a work order.
func (s *APIServer) CreateWorkOrderComment(c *gin.Context, id IdPath) {
	orderID := uuid.UUID(id)
	userID := userIDFromContext(c)

	var body struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if body.Text == "" {
		response.BadRequest(c, "text must not be empty")
		return
	}

	newID := uuid.New()
	_, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO work_order_comments (id, order_id, author_id, text, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, newID, orderID, userID, body.Text)
	if err != nil {
		response.InternalError(c, "failed to create comment")
		return
	}

	s.recordAudit(c, "order_comment.created", "maintenance", "work_order_comment", newID, map[string]any{
		"order_id": orderID.String(),
	})
	response.Created(c, gin.H{"id": newID.String()})
}
