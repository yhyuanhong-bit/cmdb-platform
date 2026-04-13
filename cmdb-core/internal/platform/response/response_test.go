package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	return c, w
}

func TestOK(t *testing.T) {
	c, w := setupTestContext()
	OK(c, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body SingleResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Data == nil {
		t.Error("expected data field in response")
	}
	if body.Meta.RequestID == "" {
		t.Error("expected non-empty request_id in meta")
	}
}

func TestCreated(t *testing.T) {
	c, w := setupTestContext()
	Created(c, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var body SingleResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Data == nil {
		t.Error("expected data field in response")
	}
}

func TestOKList(t *testing.T) {
	c, w := setupTestContext()
	items := []string{"a", "b", "c"}
	OKList(c, items, 1, 10, 3)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Pagination.Page != 1 {
		t.Errorf("expected page 1, got %d", body.Pagination.Page)
	}
	if body.Pagination.Total != 3 {
		t.Errorf("expected total 3, got %d", body.Pagination.Total)
	}
	if body.Pagination.TotalPages != 1 {
		t.Errorf("expected total_pages 1, got %d", body.Pagination.TotalPages)
	}
}

func TestOKList_ZeroPageSize(t *testing.T) {
	c, w := setupTestContext()
	OKList(c, []string{}, 1, 0, 0)

	var body ListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Pagination.TotalPages != 0 {
		t.Errorf("expected total_pages 0 for zero page_size, got %d", body.Pagination.TotalPages)
	}
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(*gin.Context, string)
		expected int
		code     string
	}{
		{"BadRequest", BadRequest, http.StatusBadRequest, "BAD_REQUEST"},
		{"NotFound", NotFound, http.StatusNotFound, "NOT_FOUND"},
		{"Unauthorized", Unauthorized, http.StatusUnauthorized, "UNAUTHORIZED"},
		{"Forbidden", Forbidden, http.StatusForbidden, "FORBIDDEN"},
		{"InternalError", InternalError, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()
			tt.fn(c, "test message")

			if w.Code != tt.expected {
				t.Errorf("expected status %d, got %d", tt.expected, w.Code)
			}

			var body ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if body.Error.Code != tt.code {
				t.Errorf("expected error code %q, got %q", tt.code, body.Error.Code)
			}
			if body.Error.Message != "test message" {
				t.Errorf("expected message %q, got %q", "test message", body.Error.Message)
			}
			if body.Meta.RequestID == "" {
				t.Error("expected non-empty request_id in meta")
			}
		})
	}
}

func TestErr(t *testing.T) {
	c, w := setupTestContext()
	Err(c, http.StatusTeapot, "TEAPOT", "i am a teapot")

	if w.Code != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, w.Code)
	}

	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if body.Error.Code != "TEAPOT" {
		t.Errorf("expected code TEAPOT, got %q", body.Error.Code)
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name         string
		queryPage    string
		querySize    string
		wantPage     int
		wantPageSize int
		wantOffset   int
	}{
		{"defaults", "", "", 1, 20, 0},
		{"page 2", "2", "10", 2, 10, 10},
		{"negative page", "-1", "10", 1, 10, 0},
		{"zero page_size", "1", "0", 1, 20, 0},
		{"excessive page_size capped", "1", "200", 1, 100, 0},
		{"invalid strings", "abc", "xyz", 1, 20, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			url := "/test"
			sep := "?"
			if tt.queryPage != "" {
				url += sep + "page=" + tt.queryPage
				sep = "&"
			}
			if tt.querySize != "" {
				url += sep + "page_size=" + tt.querySize
			}
			c.Request = httptest.NewRequest("GET", url, nil)

			page, pageSize, offset := ParsePagination(c)
			if page != tt.wantPage {
				t.Errorf("page = %d, want %d", page, tt.wantPage)
			}
			if pageSize != tt.wantPageSize {
				t.Errorf("pageSize = %d, want %d", pageSize, tt.wantPageSize)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		input    string
		def      int
		expected int
	}{
		{"", 5, 5},
		{"10", 0, 10},
		{"-3", 0, -3},
		{"abc", 7, 7},
		{"12x", 0, 0},
	}
	for _, tt := range tests {
		got := atoi(tt.input, tt.def)
		if got != tt.expected {
			t.Errorf("atoi(%q, %d) = %d, want %d", tt.input, tt.def, got, tt.expected)
		}
	}
}
