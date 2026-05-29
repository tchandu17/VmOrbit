package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// TemplateHandler handles VM template REST endpoints.
type TemplateHandler struct {
	svc     port.TemplateService
	enqueue EnqueueFunc
	log     logger.Logger
}

// NewTemplateHandler creates a new TemplateHandler.
func NewTemplateHandler(svc port.TemplateService, enqueue EnqueueFunc, log logger.Logger) *TemplateHandler {
	return &TemplateHandler{svc: svc, enqueue: enqueue, log: log}
}

// List godoc
// @Summary      List VM templates
// @Tags         templates
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Param        page           query  int     false  "Page number"
// @Param        page_size      query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /templates [get]
func (h *TemplateHandler) List(c *gin.Context) {
	result, err := h.svc.List(c.Request.Context(), c.Query("hypervisor_id"), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// GetByID godoc
// @Summary      Get template by ID
// @Tags         templates
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Template ID"
// @Success      200  {object}  APIResponse
// @Router       /templates/{id} [get]
func (h *TemplateHandler) GetByID(c *gin.Context) {
	tmpl, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "template not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, tmpl)
}

// SyncTemplates godoc
// @Summary      Trigger template sync for a hypervisor
// @Tags         templates
// @Security     BearerAuth
// @Param        hypervisor_id  path  string  true  "Hypervisor ID"
// @Success      202  {object}  APIResponse
// @Router       /hypervisors/{hypervisor_id}/templates/sync [post]
func (h *TemplateHandler) SyncTemplates(c *gin.Context) {
	hypervisorID := c.Param("id")
	taskID, err := h.svc.SyncTemplates(c.Request.Context(), hypervisorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "hypervisor not found")
			return
		}
		internalError(c, err.Error())
		return
	}

	if h.enqueue != nil {
		if err := h.enqueue(c.Request.Context(), taskID, 5); err != nil {
			h.log.Warn("failed to enqueue template sync task",
				logger.String("task_id", taskID),
				logger.Error(err),
			)
		}
	}

	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}
