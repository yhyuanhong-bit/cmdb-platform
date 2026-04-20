package api

import (
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListWorkOrderComments handles GET /maintenance/orders/{id}/comments
// Returns all comments for a specific work order, ordered oldest-first.
func (s *APIServer) ListWorkOrderComments(c *gin.Context, id IdPath) {
	orderID := uuid.UUID(id)

	rows, err := dbgen.New(s.pool).ListWorkOrderComments(c.Request.Context(), orderID)
	if err != nil {
		response.InternalError(c, "failed to query comments")
		return
	}

	comments := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		// author_name may be NULL if the user has been deleted
		// (author_id FK is ON DELETE SET NULL). The pre-migration
		// handler used a *string; preserve the JSON shape by
		// marshalling nil when not valid.
		var authorName *string
		if r.AuthorName.Valid {
			name := r.AuthorName.String
			authorName = &name
		}
		comments = append(comments, gin.H{
			"id":          r.ID.String(),
			"author_name": authorName,
			"text":        r.Text,
			"created_at":  r.CreatedAt.UTC().Format(time.RFC3339),
		})
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
	if err := dbgen.New(s.pool).CreateWorkOrderComment(c.Request.Context(), dbgen.CreateWorkOrderCommentParams{
		ID:       newID,
		OrderID:  orderID,
		AuthorID: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
		Text:     body.Text,
	}); err != nil {
		response.InternalError(c, "failed to create comment")
		return
	}

	s.recordAudit(c, "order_comment.created", "maintenance", "work_order_comment", newID, map[string]any{
		"order_id": orderID.String(),
	})
	response.Created(c, gin.H{"id": newID.String()})
}
