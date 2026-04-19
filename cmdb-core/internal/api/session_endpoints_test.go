package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TestListUserSessions_BlocksOtherUser verifies that a non-admin user cannot
// list another user's sessions (IDOR protection). An admin may list anyone's
// sessions, and a user may list their own.
func TestListUserSessions_BlocksOtherUser(t *testing.T) {
	userX := uuid.New()
	userY := uuid.New()

	tests := []struct {
		name        string
		currentUser uuid.UUID
		pathUser    uuid.UUID
		isAdmin     bool
		wantAllowed bool
	}{
		{
			name:        "regular user X requesting user Y's sessions is forbidden",
			currentUser: userX,
			pathUser:    userY,
			isAdmin:     false,
			wantAllowed: false,
		},
		{
			name:        "admin may list any user's sessions",
			currentUser: userX,
			pathUser:    userY,
			isAdmin:     true,
			wantAllowed: true,
		},
		{
			name:        "user X may list own sessions",
			currentUser: userX,
			pathUser:    userX,
			isAdmin:     false,
			wantAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("user_id", tc.currentUser.String())
			if tc.isAdmin {
				c.Set("is_admin", true)
			}
			got := canListUserSessions(c, tc.pathUser)
			if got != tc.wantAllowed {
				t.Fatalf("canListUserSessions = %v, want %v", got, tc.wantAllowed)
			}
		})
	}
}

// TestListUserSessions_Handler_ReturnsForbidden exercises the handler end to
// end and verifies the 403 response body carries the FORBIDDEN error code.
func TestListUserSessions_Handler_ReturnsForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	currentUser := uuid.New()
	targetUser := uuid.New()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("user_id", currentUser.String())
	req, _ := http.NewRequest(http.MethodGet, "/users/"+targetUser.String()+"/sessions", bytes.NewReader(nil))
	c.Request = req

	s := &APIServer{}
	s.ListUserSessions(c, IdPath(openapi_types.UUID(targetUser)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403. body=%s", rec.Code, rec.Body.String())
	}

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != "FORBIDDEN" {
		t.Errorf("error.code = %q, want FORBIDDEN", env.Error.Code)
	}
}
