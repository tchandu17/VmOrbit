package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ProviderHealthHandler handles provider health REST endpoints.
type ProviderHealthHandler struct {
	svc port.ProviderHealthService
	log logger.Logger
}

// NewProviderHealthHandler creates a new ProviderHealthHandler.
func NewProviderHealthHandler(svc port.ProviderHealthService, log logger.Logger) *ProviderHealthHandler {
	return &ProviderHealthHandler{svc: svc, log: log}
}

// ListAll godoc
// @Summary      List health snapshots for all providers
// @Tags         health
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /providers/health [get]
func (h *ProviderHealthHandler) ListAll(c *gin.Context) {
	items, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, items)
}

// GetByHypervisor godoc
// @Summary      Get health snapshot for a single provider
// @Tags         health
// @Security     BearerAuth
// @Param        id  path  string  true  "Hypervisor ID"
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /providers/{id}/health [get]
func (h *ProviderHealthHandler) GetByHypervisor(c *gin.Context) {
	snap, err := h.svc.GetByHypervisorID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, "health data not found for this hypervisor")
		return
	}
	ok(c, snap)
}

// GetHistory godoc
// @Summary      Get health history for a single provider
// @Tags         health
// @Security     BearerAuth
// @Param        id     path   string  true   "Hypervisor ID"
// @Param        limit  query  int     false  "Number of history points (default 60)"
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /providers/{id}/health/history [get]
func (h *ProviderHealthHandler) GetHistory(c *gin.Context) {
	limit := 60
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	items, err := h.svc.GetHistory(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, items)
}

// TriggerCheck godoc
// @Summary      Trigger an immediate health check for a provider
// @Tags         health
// @Security     BearerAuth
// @Param        id  path  string  true  "Hypervisor ID"
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /providers/{id}/health/check [post]
func (h *ProviderHealthHandler) TriggerCheck(c *gin.Context) {
	snap, err := h.svc.RunCheck(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, snap)
}
