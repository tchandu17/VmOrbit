package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// EnvironmentHandler handles environment and orchestration REST endpoints.
type EnvironmentHandler struct {
	svc     port.EnvironmentService
	enqueue EnqueueFunc
	log     logger.Logger
}

// NewEnvironmentHandler creates a new EnvironmentHandler.
func NewEnvironmentHandler(svc port.EnvironmentService, enqueue EnqueueFunc, log logger.Logger) *EnvironmentHandler {
	return &EnvironmentHandler{svc: svc, enqueue: enqueue, log: log}
}

// ─────────────────────────────────────────────────────────────────────────────
// Environment CRUD
// ─────────────────────────────────────────────────────────────────────────────

// List godoc
// @Summary      List environments
// @Tags         environments
// @Security     BearerAuth
// @Produce      json
// @Param        page       query  int     false  "Page number"
// @Param        page_size  query  int     false  "Page size"
// @Param        type       query  string  false  "Filter by type"
// @Param        status     query  string  false  "Filter by status"
// @Param        search     query  string  false  "Search by name/description"
// @Success      200  {object}  APIResponse
// @Router       /environments [get]
func (h *EnvironmentHandler) List(c *gin.Context) {
	filter := port.EnvironmentFilter{
		Type:   c.Query("type"),
		Status: c.Query("status"),
		Search: c.Query("search"),
	}
	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

type createEnvironmentRequest struct {
	Name        string                 `json:"name"        binding:"required"`
	Description string                 `json:"description"`
	Type        model.EnvironmentType  `json:"type"`
	OwnerID     string                 `json:"owner_id"`
	Tags        []string               `json:"tags"`
	Color       string                 `json:"color"`
	Metadata    model.JSONMap          `json:"metadata"`
}

// Create godoc
// @Summary      Create an environment
// @Tags         environments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      createEnvironmentRequest  true  "Environment details"
// @Success      201   {object}  APIResponse
// @Router       /environments [post]
func (h *EnvironmentHandler) Create(c *gin.Context) {
	var req createEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	env, err := h.svc.Create(c.Request.Context(), port.CreateEnvironmentRequest{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		OwnerID:     req.OwnerID,
		Tags:        req.Tags,
		Color:       req.Color,
		Metadata:    req.Metadata,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, env)
}

// GetByID godoc
// @Summary      Get environment by ID
// @Tags         environments
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Environment ID"
// @Success      200  {object}  APIResponse
// @Router       /environments/{id} [get]
func (h *EnvironmentHandler) GetByID(c *gin.Context) {
	env, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if isNotFound(err) {
			notFound(c, "environment not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, env)
}

type updateEnvironmentRequest struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	Type        *model.EnvironmentType `json:"type"`
	OwnerID     *string                `json:"owner_id"`
	Tags        []string               `json:"tags"`
	Color       *string                `json:"color"`
	Metadata    model.JSONMap          `json:"metadata"`
}

// Update godoc
// @Summary      Update environment
// @Tags         environments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string                    true  "Environment ID"
// @Param        body  body  updateEnvironmentRequest  true  "Update fields"
// @Success      200   {object}  APIResponse
// @Router       /environments/{id} [put]
func (h *EnvironmentHandler) Update(c *gin.Context) {
	var req updateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	env, err := h.svc.Update(c.Request.Context(), c.Param("id"), port.UpdateEnvironmentRequest{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		OwnerID:     req.OwnerID,
		Tags:        req.Tags,
		Color:       req.Color,
		Metadata:    req.Metadata,
	})
	if err != nil {
		if isNotFound(err) {
			notFound(c, "environment not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, env)
}

// Delete godoc
// @Summary      Delete environment
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      204
// @Router       /environments/{id} [delete]
func (h *EnvironmentHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if isNotFound(err) {
			notFound(c, "environment not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// ─────────────────────────────────────────────────────────────────────────────
// VM membership
// ─────────────────────────────────────────────────────────────────────────────

type addVMRequest struct {
	VMID       string `json:"vm_id"       binding:"required"`
	StartOrder int    `json:"start_order"`
	StopOrder  int    `json:"stop_order"`
	Role       string `json:"role"`
	Notes      string `json:"notes"`
}

// AddVM godoc
// @Summary      Add a VM to an environment
// @Tags         environments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string        true  "Environment ID"
// @Param        body  body  addVMRequest  true  "VM details"
// @Success      200   {object}  APIResponse
// @Router       /environments/{id}/vms [post]
func (h *EnvironmentHandler) AddVM(c *gin.Context) {
	var req addVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	if err := h.svc.AddVM(c.Request.Context(), c.Param("id"), req.VMID, port.AddVMToEnvironmentRequest{
		StartOrder: req.StartOrder,
		StopOrder:  req.StopOrder,
		Role:       req.Role,
		Notes:      req.Notes,
	}); err != nil {
		if isNotFound(err) {
			notFound(c, err.Error())
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"message": "VM added to environment"})
}

// RemoveVM godoc
// @Summary      Remove a VM from an environment
// @Tags         environments
// @Security     BearerAuth
// @Param        id    path  string  true  "Environment ID"
// @Param        vmId  path  string  true  "VM ID"
// @Success      204
// @Router       /environments/{id}/vms/{vmId} [delete]
func (h *EnvironmentHandler) RemoveVM(c *gin.Context) {
	if err := h.svc.RemoveVM(c.Request.Context(), c.Param("id"), c.Param("vmId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// ListVMs godoc
// @Summary      List VMs in an environment
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      200  {object}  APIResponse
// @Router       /environments/{id}/vms [get]
func (h *EnvironmentHandler) ListVMs(c *gin.Context) {
	vms, err := h.svc.ListVMs(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, vms)
}

type updateVMOrderingRequest struct {
	StartOrder int    `json:"start_order"`
	StopOrder  int    `json:"stop_order"`
	Role       string `json:"role"`
}

// UpdateVMOrdering godoc
// @Summary      Update VM ordering in an environment
// @Tags         environments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string                   true  "Environment ID"
// @Param        vmId  path  string                   true  "VM ID"
// @Param        body  body  updateVMOrderingRequest  true  "Ordering"
// @Success      200   {object}  APIResponse
// @Router       /environments/{id}/vms/{vmId} [put]
func (h *EnvironmentHandler) UpdateVMOrdering(c *gin.Context) {
	var req updateVMOrderingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := h.svc.UpdateVMOrdering(c.Request.Context(), c.Param("id"), c.Param("vmId"), port.UpdateVMOrderingRequest{
		StartOrder: req.StartOrder,
		StopOrder:  req.StopOrder,
		Role:       req.Role,
	}); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"message": "ordering updated"})
}

// ─────────────────────────────────────────────────────────────────────────────
// Dependencies
// ─────────────────────────────────────────────────────────────────────────────

type addDependencyRequest struct {
	SourceVMID   string               `json:"source_vm_id"  binding:"required"`
	TargetVMID   string               `json:"target_vm_id"  binding:"required"`
	Type         model.DependencyType `json:"type"          binding:"required"`
	DelaySeconds int                  `json:"delay_seconds"`
	Notes        string               `json:"notes"`
}

// AddDependency godoc
// @Summary      Add a VM dependency
// @Tags         environments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string               true  "Environment ID"
// @Param        body  body  addDependencyRequest true  "Dependency"
// @Success      201   {object}  APIResponse
// @Router       /environments/{id}/dependencies [post]
func (h *EnvironmentHandler) AddDependency(c *gin.Context) {
	var req addDependencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	dep, err := h.svc.AddDependency(c.Request.Context(), c.Param("id"), port.AddDependencyRequest{
		SourceVMID:   req.SourceVMID,
		TargetVMID:   req.TargetVMID,
		Type:         req.Type,
		DelaySeconds: req.DelaySeconds,
		Notes:        req.Notes,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, dep)
}

// RemoveDependency godoc
// @Summary      Remove a VM dependency
// @Tags         environments
// @Security     BearerAuth
// @Param        id   path  string  true  "Environment ID"
// @Param        depId path string  true  "Dependency ID"
// @Success      204
// @Router       /environments/{id}/dependencies/{depId} [delete]
func (h *EnvironmentHandler) RemoveDependency(c *gin.Context) {
	if err := h.svc.RemoveDependency(c.Request.Context(), c.Param("depId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// ListDependencies godoc
// @Summary      List VM dependencies for an environment
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      200  {object}  APIResponse
// @Router       /environments/{id}/dependencies [get]
func (h *EnvironmentHandler) ListDependencies(c *gin.Context) {
	deps, err := h.svc.ListDependencies(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, deps)
}

// ─────────────────────────────────────────────────────────────────────────────
// Orchestration operations
// ─────────────────────────────────────────────────────────────────────────────

// Start godoc
// @Summary      Start all VMs in an environment (dependency-aware)
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      202  {object}  APIResponse
// @Router       /environments/{id}/start [post]
func (h *EnvironmentHandler) Start(c *gin.Context) {
	h.triggerOrchestration(c, func(ctx context.Context, id string) (string, error) {
		return h.svc.StartEnvironment(ctx, id)
	})
}

// Stop godoc
// @Summary      Stop all VMs in an environment (reverse dependency order)
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      202  {object}  APIResponse
// @Router       /environments/{id}/stop [post]
func (h *EnvironmentHandler) Stop(c *gin.Context) {
	h.triggerOrchestration(c, func(ctx context.Context, id string) (string, error) {
		return h.svc.StopEnvironment(ctx, id)
	})
}

// Restart godoc
// @Summary      Restart all VMs in an environment
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      202  {object}  APIResponse
// @Router       /environments/{id}/restart [post]
func (h *EnvironmentHandler) Restart(c *gin.Context) {
	h.triggerOrchestration(c, func(ctx context.Context, id string) (string, error) {
		return h.svc.RestartEnvironment(ctx, id)
	})
}

type snapshotEnvironmentRequest struct {
	SnapshotName string `json:"snapshot_name" binding:"required"`
	Description  string `json:"description"`
	Memory       bool   `json:"memory"`
}

// Snapshot godoc
// @Summary      Snapshot all VMs in an environment
// @Tags         environments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string                      true  "Environment ID"
// @Param        body  body  snapshotEnvironmentRequest  true  "Snapshot params"
// @Success      202   {object}  APIResponse
// @Router       /environments/{id}/snapshot [post]
func (h *EnvironmentHandler) Snapshot(c *gin.Context) {
	var req snapshotEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	taskID, err := h.svc.SnapshotEnvironment(c.Request.Context(), c.Param("id"), port.EnvironmentSnapshotRequest{
		SnapshotName: req.SnapshotName,
		Description:  req.Description,
		Memory:       req.Memory,
	})
	if err != nil {
		if isNotFound(err) {
			notFound(c, "environment not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	h.enqueueAndRespond(c, taskID)
}

type cloneEnvironmentRequest struct {
	NewEnvironmentName string `json:"new_environment_name" binding:"required"`
	NameSuffix         string `json:"name_suffix"`
}

// Clone godoc
// @Summary      Clone all VMs in an environment into a new environment
// @Tags         environments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string                    true  "Environment ID"
// @Param        body  body  cloneEnvironmentRequest   true  "Clone params"
// @Success      202   {object}  APIResponse
// @Router       /environments/{id}/clone [post]
func (h *EnvironmentHandler) Clone(c *gin.Context) {
	var req cloneEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	taskID, err := h.svc.CloneEnvironment(c.Request.Context(), c.Param("id"), port.EnvironmentCloneRequest{
		NewEnvironmentName: req.NewEnvironmentName,
		NameSuffix:         req.NameSuffix,
	})
	if err != nil {
		if isNotFound(err) {
			notFound(c, "environment not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	h.enqueueAndRespond(c, taskID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Run tracking
// ─────────────────────────────────────────────────────────────────────────────

// ListRuns godoc
// @Summary      List orchestration runs for an environment
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      200  {object}  APIResponse
// @Router       /environments/{id}/runs [get]
func (h *EnvironmentHandler) ListRuns(c *gin.Context) {
	result, err := h.svc.ListRuns(c.Request.Context(), c.Param("id"), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// GetRun godoc
// @Summary      Get an orchestration run by ID
// @Tags         environments
// @Security     BearerAuth
// @Param        id     path  string  true  "Environment ID"
// @Param        runId  path  string  true  "Run ID"
// @Success      200  {object}  APIResponse
// @Router       /environments/{id}/runs/{runId} [get]
func (h *EnvironmentHandler) GetRun(c *gin.Context) {
	run, err := h.svc.GetRun(c.Request.Context(), c.Param("runId"))
	if err != nil {
		if isNotFound(err) {
			notFound(c, "run not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, run)
}

// GetRunSteps godoc
// @Summary      Get steps for an orchestration run
// @Tags         environments
// @Security     BearerAuth
// @Param        id     path  string  true  "Environment ID"
// @Param        runId  path  string  true  "Run ID"
// @Success      200  {object}  APIResponse
// @Router       /environments/{id}/runs/{runId}/steps [get]
func (h *EnvironmentHandler) GetRunSteps(c *gin.Context) {
	steps, err := h.svc.GetRunSteps(c.Request.Context(), c.Param("runId"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, steps)
}

// RefreshStatus godoc
// @Summary      Refresh aggregate environment status
// @Tags         environments
// @Security     BearerAuth
// @Param        id  path  string  true  "Environment ID"
// @Success      200  {object}  APIResponse
// @Router       /environments/{id}/refresh-status [post]
func (h *EnvironmentHandler) RefreshStatus(c *gin.Context) {
	env, err := h.svc.RefreshStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		if isNotFound(err) {
			notFound(c, "environment not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, env)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func (h *EnvironmentHandler) triggerOrchestration(c *gin.Context, fn func(context.Context, string) (string, error)) {
	taskID, err := fn(c.Request.Context(), c.Param("id"))
	if err != nil {
		if isNotFound(err) {
			notFound(c, "environment not found")
			return
		}
		if strings.Contains(err.Error(), "no VMs") {
			badRequest(c, err.Error())
			return
		}
		internalError(c, err.Error())
		return
	}
	h.enqueueAndRespond(c, taskID)
}

func (h *EnvironmentHandler) enqueueAndRespond(c *gin.Context, taskID string) {
	if h.enqueue != nil {
		if err := h.enqueue(c.Request.Context(), taskID, 5); err != nil {
			h.log.Warn("failed to enqueue orchestration task",
				logger.String("task_id", taskID),
				logger.Error(err),
			)
		}
	}
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound) ||
		strings.Contains(err.Error(), "not found")
}
