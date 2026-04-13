package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// userSession represents one session record returned by GetUserSessions.
type userSession struct {
	ID         string  `json:"id"`
	IPAddress  string  `json:"ip"`
	Device     string  `json:"device"`
	Browser    string  `json:"browser"`
	Icon       string  `json:"icon"`
	Time       string  `json:"time"`
	LastActive string  `json:"lastActive"`
	Current    bool    `json:"current"`
}

// deviceIcon maps a device_type string to a Material icon name.
func deviceIcon(deviceType string) string {
	switch deviceType {
	case "desktop":
		return "laptop_mac"
	case "mobile":
		return "phone_iphone"
	case "tablet":
		return "tablet_mac"
	default:
		return "devices"
	}
}

// GetUserSessions handles GET /users/:id/sessions
// Returns the 20 most recent sessions for a given user.
func (s *APIServer) GetUserSessions(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user id"})
		return
	}

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT id, ip_address, device_type, browser, created_at, last_active_at, is_current
		FROM user_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query user sessions"})
		return
	}
	defer rows.Close()

	sessions := []userSession{}
	for rows.Next() {
		var (
			id           string
			ipAddress    string
			deviceType   string
			browser      string
			createdAt    time.Time
			lastActiveAt time.Time
			isCurrent    bool
		)
		if err := rows.Scan(&id, &ipAddress, &deviceType, &browser, &createdAt, &lastActiveAt, &isCurrent); err != nil {
			continue
		}
		sessions = append(sessions, userSession{
			ID:         id,
			IPAddress:  ipAddress,
			Device:     deviceType,
			Browser:    browser,
			Icon:       deviceIcon(deviceType),
			Time:       createdAt.Format(time.RFC3339),
			LastActive: lastActiveAt.Format(time.RFC3339),
			Current:    isCurrent,
		})
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// ChangePassword handles POST /auth/change-password
// Validates the current password and sets a new one for the authenticated user.
func (s *APIServer) ChangePassword(c *gin.Context) {
	userID := userIDFromContext(c)

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.CurrentPassword == "" || body.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_password and new_password are required"})
		return
	}

	if err := s.authSvc.ChangePassword(c.Request.Context(), userID, body.CurrentPassword, body.NewPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}
