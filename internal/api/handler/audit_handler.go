package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// AuditHandler handles audit log REST endpoints.
type AuditHandler struct {
	svc port.AuditService
	log logger.Logger
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(svc port.AuditService, log logger.Logger) *AuditHandler {
	return &AuditHandler{svc: svc, log: log}
}

// List godoc
// @Summary      List audit logs
// @Tags         audit
// @Security     BearerAuth
// @Produce      json
// @Param        user_id        query  string  false  "Filter by user ID (UUID)"
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor ID (UUID)"
// @Param        resource       query  string  false  "Filter by resource type"
// @Param        resource_id    query  string  false  "Filter by resource ID (UUID)"
// @Param        action         query  string  false  "Filter by action"
// @Param        since          query  string  false  "Filter from timestamp (RFC3339)"
// @Param        until          query  string  false  "Filter to timestamp (RFC3339)"
// @Param        success        query  bool    false  "Filter by success status"
// @Param        search         query  string  false  "Full-text search on description/username"
// @Param        page           query  int     false  "Page number"
// @Param        page_size      query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /audit [get]
func (h *AuditHandler) List(c *gin.Context) {
	filter := buildAuditFilter(c)
	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// Export godoc
// @Summary      Export audit logs as CSV
// @Tags         audit
// @Security     BearerAuth
// @Produce      text/csv
// @Param        user_id        query  string  false  "Filter by user ID (UUID)"
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor ID (UUID)"
// @Param        resource       query  string  false  "Filter by resource type"
// @Param        action         query  string  false  "Filter by action"
// @Param        since          query  string  false  "Filter from timestamp (RFC3339)"
// @Param        until          query  string  false  "Filter to timestamp (RFC3339)"
// @Param        success        query  bool    false  "Filter by success status"
// @Success      200
// @Router       /audit/export [get]
func (h *AuditHandler) Export(c *gin.Context) {
	filter := buildAuditFilter(c)
	// Export up to 10 000 rows
	result, err := h.svc.List(c.Request.Context(), filter, port.Page{Number: 1, Size: 10000})
	if err != nil {
		internalError(c, err.Error())
		return
	}

	filename := fmt.Sprintf("audit_export_%s.csv", time.Now().UTC().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Status(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	// Header row
	_ = w.Write([]string{
		"id", "created_at", "username", "user_id",
		"action", "resource", "resource_id", "description",
		"hypervisor_id", "ip_address", "success", "error_message", "request_id",
	})

	for _, entry := range result.Items {
		userID := ""
		if entry.UserID != nil {
			userID = entry.UserID.String()
		}
		resourceID := ""
		if entry.ResourceID != nil {
			resourceID = entry.ResourceID.String()
		}
		hypervisorID := ""
		if entry.HypervisorID != nil {
			hypervisorID = entry.HypervisorID.String()
		}
		success := "false"
		if entry.Success {
			success = "true"
		}
		_ = w.Write([]string{
			entry.ID.String(),
			entry.CreatedAt.UTC().Format(time.RFC3339),
			entry.Username,
			userID,
			string(entry.Action),
			entry.Resource,
			resourceID,
			entry.Description,
			hypervisorID,
			entry.IPAddress,
			success,
			entry.ErrorMessage,
			entry.RequestID,
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildAuditFilter(c *gin.Context) port.AuditFilter {
	filter := port.AuditFilter{
		Resource: c.Query("resource"),
		Action:   model.AuditAction(c.Query("action")),
		Search:   c.Query("search"),
	}

	if raw := c.Query("user_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.UserID = &id
		}
	}
	if raw := c.Query("hypervisor_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.HypervisorID = &id
		}
	}
	if raw := c.Query("resource_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.ResourceID = &id
		}
	}
	if raw := c.Query("since"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			filter.Since = &t
		}
	}
	if raw := c.Query("until"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			filter.Until = &t
		}
	}
	if raw := c.Query("success"); raw != "" {
		v := raw == "true"
		filter.SuccessOnly = &v
	}

	return filter
}
