package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ProvisioningHandler handles VM clone and provision REST endpoints.
type ProvisioningHandler struct {
	svc     port.ProvisioningService
	enqueue EnqueueFunc
	log     logger.Logger
}

// NewProvisioningHandler creates a new ProvisioningHandler.
func NewProvisioningHandler(svc port.ProvisioningService, enqueue EnqueueFunc, log logger.Logger) *ProvisioningHandler {
	return &ProvisioningHandler{svc: svc, enqueue: enqueue, log: log}
}

// enqueueTask pushes a task onto the Redis queue immediately after creation.
func (h *ProvisioningHandler) enqueueTask(c *gin.Context, taskID string, priority int) {
	if h.enqueue == nil {
		return
	}
	if err := h.enqueue(c.Request.Context(), taskID, priority); err != nil {
		h.log.Warn("failed to enqueue provisioning task into Redis (will be picked up by poller)",
			logger.String("task_id", taskID),
			logger.Error(err),
		)
	}
}

// ── Clone ─────────────────────────────────────────────────────────────────────

type cloneVMRequest struct {
	SourceVMID string   `json:"source_vm_id" binding:"required"`
	Name       string   `json:"name"         binding:"required"`
	DataStore  string   `json:"data_store"`
	Node       string   `json:"node"`
	Linked     bool     `json:"linked"`
	Tags       []string `json:"tags"`
}

// CloneVM godoc
// @Summary      Clone a VM
// @Tags         provisioning
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  cloneVMRequest  true  "Clone spec"
// @Success      202   {object}  APIResponse
// @Router       /vms/clone [post]
func (h *ProvisioningHandler) CloneVM(c *gin.Context) {
	var req cloneVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	job, err := h.svc.Clone(c.Request.Context(), port.CloneVMRequest{
		SourceVMID: req.SourceVMID,
		Name:       req.Name,
		DataStore:  req.DataStore,
		Node:       req.Node,
		Linked:     req.Linked,
		Tags:       req.Tags,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			notFound(c, err.Error())
			return
		}
		internalError(c, err.Error())
		return
	}

	// Enqueue the task immediately.
	if job.TaskID != nil {
		h.enqueueTask(c, job.TaskID.String(), 4)
	}

	c.JSON(202, APIResponse{Success: true, Data: job})
}

// ── Provision ─────────────────────────────────────────────────────────────────

type provisionVMRequest struct {
	TemplateID  string   `json:"template_id"  binding:"required"`
	Name        string   `json:"name"         binding:"required"`
	CPUCount    int      `json:"cpu_count"`
	MemoryMB    int      `json:"memory_mb"`
	DiskGB      int      `json:"disk_gb"`
	NetworkName string   `json:"network_name"`
	DataStore   string   `json:"data_store"`
	Node        string   `json:"node"`
	Tags        []string `json:"tags"`
}

// ProvisionVM godoc
// @Summary      Provision a VM from a template
// @Tags         provisioning
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  provisionVMRequest  true  "Provision spec"
// @Success      202   {object}  APIResponse
// @Router       /vms/provision [post]
func (h *ProvisioningHandler) ProvisionVM(c *gin.Context) {
	var req provisionVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	job, err := h.svc.Provision(c.Request.Context(), port.ProvisionVMRequest{
		TemplateID:  req.TemplateID,
		Name:        req.Name,
		CPUCount:    req.CPUCount,
		MemoryMB:    req.MemoryMB,
		DiskGB:      req.DiskGB,
		NetworkName: req.NetworkName,
		DataStore:   req.DataStore,
		Node:        req.Node,
		Tags:        req.Tags,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			notFound(c, err.Error())
			return
		}
		internalError(c, err.Error())
		return
	}

	// Enqueue the task immediately.
	if job.TaskID != nil {
		h.enqueueTask(c, job.TaskID.String(), 4)
	}

	c.JSON(202, APIResponse{Success: true, Data: job})
}

// ── Provisioning Jobs ─────────────────────────────────────────────────────────

// ListJobs godoc
// @Summary      List provisioning jobs
// @Tags         provisioning
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Param        type           query  string  false  "Filter by type (clone|provision)"
// @Param        status         query  string  false  "Filter by status"
// @Param        page           query  int     false  "Page number"
// @Param        page_size      query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /provisioning-jobs [get]
func (h *ProvisioningHandler) ListJobs(c *gin.Context) {
	filter := port.ProvisioningJobFilter{
		HypervisorID: c.Query("hypervisor_id"),
		Type:         c.Query("type"),
		Status:       c.Query("status"),
	}
	result, err := h.svc.ListJobs(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// GetJob godoc
// @Summary      Get provisioning job by ID
// @Tags         provisioning
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Job ID"
// @Success      200  {object}  APIResponse
// @Router       /provisioning-jobs/{id} [get]
func (h *ProvisioningHandler) GetJob(c *gin.Context) {
	job, err := h.svc.GetJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "provisioning job not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, job)
}
