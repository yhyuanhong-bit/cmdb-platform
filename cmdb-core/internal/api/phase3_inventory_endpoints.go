package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetItemScanHistory handles GET /inventory/tasks/:id/items/:itemId/scan-history
// Returns the scan history for a specific inventory item.
func (s *APIServer) GetItemScanHistory(c *gin.Context) {
	itemIDStr := c.Param("itemId")
	itemID, err := uuid.Parse(itemIDStr)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query scan history"})
		return
	}
	defer rows.Close()

	history := []gin.H{}
	for rows.Next() {
		var (
			id          uuid.UUID
			scannedAt   time.Time
			displayName *string
			method      string
			result      string
			note        *string
		)
		if err := rows.Scan(&id, &scannedAt, &displayName, &method, &result, &note); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan row"})
			return
		}
		history = append(history, gin.H{
			"id":        id.String(),
			"timestamp": scannedAt.UTC().Format(time.RFC3339),
			"operator":  displayName,
			"method":    method,
			"result":    result,
			"note":      note,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error reading scan history rows"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"scan_history": history})
}

// CreateItemScanRecord handles POST /inventory/tasks/:id/items/:itemId/scan-history
// Creates a new scan record for a specific inventory item.
func (s *APIServer) CreateItemScanRecord(c *gin.Context) {
	itemIDStr := c.Param("itemId")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	userID := userIDFromContext(c)

	var body struct {
		Method string  `json:"method" binding:"required"`
		Result string  `json:"result" binding:"required"`
		Note   *string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newID := uuid.New()
	_, err = s.pool.Exec(c.Request.Context(), `
		INSERT INTO inventory_scan_history (id, item_id, scanned_by, method, result, note, scanned_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`, newID, itemID, userID, body.Method, body.Result, body.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create scan record"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": newID.String()})
}

// GetItemNotes handles GET /inventory/tasks/:id/items/:itemId/notes
// Returns notes for a specific inventory item.
func (s *APIServer) GetItemNotes(c *gin.Context) {
	itemIDStr := c.Param("itemId")
	itemID, err := uuid.Parse(itemIDStr)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query notes"})
		return
	}
	defer rows.Close()

	notes := []gin.H{}
	for rows.Next() {
		var (
			id          uuid.UUID
			createdAt   time.Time
			displayName *string
			severity    string
			text        string
		)
		if err := rows.Scan(&id, &createdAt, &displayName, &severity, &text); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan row"})
			return
		}
		notes = append(notes, gin.H{
			"id":        id.String(),
			"timestamp": createdAt.UTC().Format(time.RFC3339),
			"author":    displayName,
			"severity":  severity,
			"text":      text,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error reading notes rows"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notes": notes})
}

// CreateItemNote handles POST /inventory/tasks/:id/items/:itemId/notes
// Creates a new note for a specific inventory item.
func (s *APIServer) CreateItemNote(c *gin.Context) {
	itemIDStr := c.Param("itemId")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}

	userID := userIDFromContext(c)

	var body struct {
		Severity string `json:"severity"`
		Text     string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Severity == "" {
		body.Severity = "info"
	}

	newID := uuid.New()
	_, err = s.pool.Exec(c.Request.Context(), `
		INSERT INTO inventory_notes (id, item_id, author_id, severity, text, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
	`, newID, itemID, userID, body.Severity, body.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create note"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": newID.String()})
}
