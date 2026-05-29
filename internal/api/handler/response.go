package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// APIResponse is the standard envelope for all API responses.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta carries pagination metadata.
type Meta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: data})
}

func created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{Success: true, Data: data})
}

func noContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: msg})
}

func unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Error: msg})
}

func forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, APIResponse{Success: false, Error: msg})
}

func notFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: msg})
}

func internalError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: msg})
}

func paginated[T any](c *gin.Context, result *port.PageResult[T]) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result.Items,
		Meta: &Meta{
			Page:       result.Page,
			PageSize:   result.PageSize,
			TotalItems: result.TotalItems,
			TotalPages: result.TotalPages,
		},
	})
}

// parsePage extracts page/page_size query params with sensible defaults.
func parsePage(c *gin.Context) port.Page {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return port.Page{Number: page, Size: size}
}
