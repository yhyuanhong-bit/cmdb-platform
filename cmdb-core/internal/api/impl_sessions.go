package api

import (
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// hasAdminRole reports whether the current request is authenticated as an
// admin. The RBAC middleware sets "is_admin" on the Gin context when the
// merged permissions grant wildcard access ("*":["*"]). Tests may also set
// the flag directly.
func hasAdminRole(c *gin.Context) bool {
	v, ok := c.Get("is_admin")
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// canListUserSessions enforces that the caller is either listing their own
// sessions or has admin privileges. Extracted so the authorization decision
// is unit-testable without a database.
func canListUserSessions(c *gin.Context, pathUserID uuid.UUID) bool {
	currentUserID := userIDFromContext(c)
	if pathUserID == currentUserID {
		return true
	}
	return hasAdminRole(c)
}

// userSession represents one session record returned by GetUserSessions.
type userSession struct {
	ID         string `json:"id"`
	IPAddress  string `json:"ip"`
	Device     string `json:"device"`
	Browser    string `json:"browser"`
	Icon       string `json:"icon"`
	Time       string `json:"time"`
	LastActive string `json:"lastActive"`
	Current    bool   `json:"current"`
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

// ListUserSessions handles GET /users/:id/sessions
// Returns the 20 most recent sessions for a given user.
// Authorization: caller must be listing their own sessions OR be an admin.
func (s *APIServer) ListUserSessions(c *gin.Context, id IdPath) {
	userID := uuid.UUID(id)

	if !canListUserSessions(c, userID) {
		response.Forbidden(c, "cannot list other users' sessions")
		return
	}

	rows, err := dbgen.New(s.pool).ListUserSessions(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to query user sessions")
		return
	}

	sessions := make([]userSession, 0, len(rows))
	for _, r := range rows {
		// ip_address, device_type, browser are nullable in the schema
		// so we collapse NULLs to "" to preserve the pre-migration JSON
		// shape (always a string, never null).
		deviceType := ""
		if r.DeviceType.Valid {
			deviceType = r.DeviceType.String
		}
		ip := ""
		if r.IpAddress.Valid {
			ip = r.IpAddress.String
		}
		browser := ""
		if r.Browser.Valid {
			browser = r.Browser.String
		}
		sessions = append(sessions, userSession{
			ID:         r.ID.String(),
			IPAddress:  ip,
			Device:     deviceType,
			Browser:    browser,
			Icon:       deviceIcon(deviceType),
			Time:       r.CreatedAt.Format(time.RFC3339),
			LastActive: r.LastActiveAt.Format(time.RFC3339),
			Current:    r.IsCurrent,
		})
	}

	response.OK(c, gin.H{"sessions": sessions})
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
		response.BadRequest(c, err.Error())
		return
	}
	if body.CurrentPassword == "" || body.NewPassword == "" {
		response.BadRequest(c, "current_password and new_password are required")
		return
	}

	if err := s.authSvc.ChangePassword(c.Request.Context(), userID, body.CurrentPassword, body.NewPassword); err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, gin.H{"message": "password changed successfully"})
}
