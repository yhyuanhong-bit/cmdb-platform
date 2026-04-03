package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Meta contains request metadata included in every response.
type Meta struct {
	RequestID string `json:"request_id"`
}

// SingleResponse is the envelope for a single resource.
type SingleResponse struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

// Pagination describes page position within a list result.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ListResponse is the envelope for a list of resources.
type ListResponse struct {
	Data       any        `json:"data"`
	Pagination Pagination `json:"pagination"`
	Meta       Meta       `json:"meta"`
}

// ErrorBody carries a machine-readable code and human-readable message.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the envelope for error responses.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

// OK responds with 200 and a single resource.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, SingleResponse{
		Data: data,
		Meta: Meta{RequestID: requestID(c)},
	})
}

// OKList responds with 200 and a paginated list.
func OKList(c *gin.Context, data any, page, pageSize, total int) {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	c.JSON(http.StatusOK, ListResponse{
		Data: data,
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
		Meta: Meta{RequestID: requestID(c)},
	})
}

// Created responds with 201 and a single resource.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, SingleResponse{
		Data: data,
		Meta: Meta{RequestID: requestID(c)},
	})
}

// Err responds with the given HTTP status and error details.
func Err(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message},
		Meta:  Meta{RequestID: requestID(c)},
	})
}

// BadRequest responds with 400.
func BadRequest(c *gin.Context, msg string) {
	Err(c, http.StatusBadRequest, "BAD_REQUEST", msg)
}

// NotFound responds with 404.
func NotFound(c *gin.Context, msg string) {
	Err(c, http.StatusNotFound, "NOT_FOUND", msg)
}

// Unauthorized responds with 401.
func Unauthorized(c *gin.Context, msg string) {
	Err(c, http.StatusUnauthorized, "UNAUTHORIZED", msg)
}

// Forbidden responds with 403.
func Forbidden(c *gin.Context, msg string) {
	Err(c, http.StatusForbidden, "FORBIDDEN", msg)
}

// InternalError responds with 500.
func InternalError(c *gin.Context, msg string) {
	Err(c, http.StatusInternalServerError, "INTERNAL_ERROR", msg)
}

// ParsePagination reads page and page_size query params with sensible defaults.
// Returns page, pageSize, offset.
func ParsePagination(c *gin.Context) (int, int, int) {
	page := atoi(c.Query("page"), 1)
	pageSize := atoi(c.Query("page_size"), 20)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	return page, pageSize, offset
}

// requestID reads the request ID from gin context or generates a new one.
func requestID(c *gin.Context) string {
	if id, ok := c.Get("request_id"); ok {
		if s, ok := id.(string); ok && s != "" {
			return s
		}
	}
	return uuid.New().String()
}

// atoi parses a string to int, returning def on failure.
func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	neg := false
	i := 0
	if s[0] == '-' {
		neg = true
		i = 1
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return def
		}
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		n = -n
	}
	return n
}
