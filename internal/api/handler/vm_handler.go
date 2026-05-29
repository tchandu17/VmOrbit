package handler

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// VMHandler handles virtual machine REST endpoints.
type VMHandler struct {
	svc     port.VMService
	taskSvc port.TaskService
	auditSvc port.AuditService
	enqueue EnqueueFunc
	log     logger.Logger
}

// NewVMHandler creates a new VMHandler.
// enqueue may be nil — if so, the task engine's DB fallback poller will pick
// up the task within poll_interval (typically 5 s).
func NewVMHandler(svc port.VMService, taskSvc port.TaskService, auditSvc port.AuditService, enqueue EnqueueFunc, log logger.Logger) *VMHandler {
	return &VMHandler{svc: svc, taskSvc: taskSvc, auditSvc: auditSvc, enqueue: enqueue, log: log}
}

// enqueueTask pushes a task onto the Redis queue immediately after creation.
// Non-fatal: the DB fallback poller will recover it if this fails.
func (h *VMHandler) enqueueTask(ctx context.Context, taskID string, priority int) {
	if h.enqueue == nil {
		return
	}
	if err := h.enqueue(ctx, taskID, priority); err != nil {
		h.log.Warn("failed to enqueue VM task into Redis (will be picked up by poller)",
			logger.String("task_id", taskID),
			logger.Error(err),
		)
	}
}

// List godoc
// @Summary      List VMs
// @Tags         vms
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query     string  false  "Filter by hypervisor"
// @Param        tag_ids        query     string  false  "Comma-separated tag IDs to filter by"
// @Param        status         query     string  false  "Filter by VM status"
// @Param        page           query     int     false  "Page number"
// @Param        page_size      query     int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /vms [get]
func (h *VMHandler) List(c *gin.Context) {
	filter := port.VMFilter{
		HypervisorID: c.Query("hypervisor_id"),
		Status:       c.Query("status"),
	}
	// tag_ids is a comma-separated list of tag UUIDs.
	if raw := c.Query("tag_ids"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			if id = strings.TrimSpace(id); id != "" {
				filter.TagIDs = append(filter.TagIDs, id)
			}
		}
	}
	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// GetByID godoc
// @Summary      Get VM by ID
// @Tags         vms
// @Security     BearerAuth
// @Produce      json
// @Param        id  path      string  true  "VM ID"
// @Success      200  {object}  APIResponse
// @Router       /vms/{id} [get]
func (h *VMHandler) GetByID(c *gin.Context) {
	vm, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, vm)
}

// PowerOn godoc
// @Summary      Power on a VM
// @Tags         vms
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      202  {object}  APIResponse
// @Router       /vms/{id}/power-on [post]
func (h *VMHandler) PowerOn(c *gin.Context) {
	taskID, err := h.svc.PowerOn(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueTask(c.Request.Context(), taskID, 3) // power ops get higher priority (3 < 5)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// PowerOff godoc
// @Summary      Power off a VM
// @Tags         vms
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      202  {object}  APIResponse
// @Router       /vms/{id}/power-off [post]
func (h *VMHandler) PowerOff(c *gin.Context) {
	taskID, err := h.svc.PowerOff(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueTask(c.Request.Context(), taskID, 3)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// Reboot godoc
// @Summary      Reboot a VM
// @Tags         vms
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      202  {object}  APIResponse
// @Router       /vms/{id}/reboot [post]
func (h *VMHandler) Reboot(c *gin.Context) {
	taskID, err := h.svc.Reboot(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueTask(c.Request.Context(), taskID, 3)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// Suspend godoc
// @Summary      Suspend a VM
// @Tags         vms
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      202  {object}  APIResponse
// @Router       /vms/{id}/suspend [post]
func (h *VMHandler) Suspend(c *gin.Context) {
	taskID, err := h.svc.Suspend(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueTask(c.Request.Context(), taskID, 3)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// GetMetrics godoc
// @Summary      Get VM performance metrics
// @Tags         vms
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      200  {object}  APIResponse
// @Router       /vms/{id}/metrics [get]
func (h *VMHandler) GetMetrics(c *gin.Context) {
	metrics, err := h.svc.GetMetrics(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, metrics)
}

type createSnapshotRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	Memory      bool   `json:"memory"`
	Quiesce     bool   `json:"quiesce"`
}

// ListSnapshots godoc
// @Summary      List VM snapshots
// @Tags         vms
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      200  {object}  APIResponse
// @Router       /vms/{id}/snapshots [get]
func (h *VMHandler) ListSnapshots(c *gin.Context) {
	snaps, err := h.svc.ListSnapshots(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, snaps)
}

// CreateSnapshot godoc
// @Summary      Create a VM snapshot
// @Tags         vms
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                 true  "VM ID"
// @Param        body  body      createSnapshotRequest  true  "Snapshot spec"
// @Success      202   {object}  APIResponse
// @Router       /vms/{id}/snapshots [post]
func (h *VMHandler) CreateSnapshot(c *gin.Context) {
	var req createSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	taskID, err := h.svc.CreateSnapshot(c.Request.Context(), c.Param("id"), port.SnapshotSpec{
		Name:        req.Name,
		Description: req.Description,
		Memory:      req.Memory,
		Quiesce:     req.Quiesce,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueTask(c.Request.Context(), taskID, 5) // snapshot ops at default priority
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// DeleteSnapshot godoc
// @Summary      Delete a VM snapshot
// @Tags         vms
// @Security     BearerAuth
// @Param        id          path  string  true  "VM ID"
// @Param        snapshotId  path  string  true  "Snapshot ID"
// @Success      202  {object}  APIResponse
// @Router       /vms/{id}/snapshots/{snapshotId} [delete]
func (h *VMHandler) DeleteSnapshot(c *gin.Context) {
	taskID, err := h.svc.DeleteSnapshot(c.Request.Context(), c.Param("id"), c.Param("snapshotId"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueTask(c.Request.Context(), taskID, 5)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// RevertSnapshot godoc
// @Summary      Revert VM to a snapshot
// @Tags         vms
// @Security     BearerAuth
// @Param        id          path  string  true  "VM ID"
// @Param        snapshotId  path  string  true  "Snapshot ID"
// @Success      202  {object}  APIResponse
// @Router       /vms/{id}/snapshots/{snapshotId}/revert [post]
func (h *VMHandler) RevertSnapshot(c *gin.Context) {
	taskID, err := h.svc.RevertSnapshot(c.Request.Context(), c.Param("id"), c.Param("snapshotId"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueTask(c.Request.Context(), taskID, 5)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// Delete godoc
// @Summary      Delete a VM
// @Tags         vms
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      204  {object}  APIResponse
// @Router       /vms/{id} [delete]
func (h *VMHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	c.JSON(204, APIResponse{Success: true})
}

// ListTasks godoc
// @Summary      List tasks for a VM
// @Tags         vms
// @Security     BearerAuth
// @Produce      json
// @Param        id         path   string  true   "VM ID"
// @Param        page       query  int     false  "Page number"
// @Param        page_size  query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /vms/{id}/tasks [get]
func (h *VMHandler) ListTasks(c *gin.Context) {
	result, err := h.taskSvc.ListByVMID(c.Request.Context(), c.Param("id"), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// ListActivity godoc
// @Summary      List audit activity for a VM
// @Tags         vms
// @Security     BearerAuth
// @Produce      json
// @Param        id         path   string  true   "VM ID"
// @Param        page       query  int     false  "Page number"
// @Param        page_size  query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /vms/{id}/activity [get]
func (h *VMHandler) ListActivity(c *gin.Context) {
	vmID := c.Param("id")
	id, err := uuid.Parse(vmID)
	if err != nil {
		badRequest(c, "invalid vm id")
		return
	}
	result, err := h.auditSvc.List(c.Request.Context(), port.AuditFilter{
		Resource:   "vm",
		ResourceID: &id,
	}, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// ─────────────────────────────────────────────────────────────────────────────
// Bulk operations
// ─────────────────────────────────────────────────────────────────────────────

type bulkActionRequest struct {
	VMIDs []string `json:"vm_ids" binding:"required,min=1"`
}

type bulkSnapshotRequest struct {
	VMIDs       []string `json:"vm_ids"       binding:"required,min=1"`
	Name        string   `json:"name"         binding:"required"`
	Description string   `json:"description"`
	Memory      bool     `json:"memory"`
}

// BulkPowerOn godoc
// @Summary      Bulk power on VMs
// @Tags         vms
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  bulkActionRequest  true  "VM IDs"
// @Success      202  {object}  APIResponse
// @Router       /vms/bulk/poweron [post]
func (h *VMHandler) BulkPowerOn(c *gin.Context) {
	var req bulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	parentTaskID, err := h.svc.BulkPowerOn(c.Request.Context(), req.VMIDs)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueChildTasks(c.Request.Context(), parentTaskID, req.VMIDs, 3)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": parentTaskID, "vm_count": len(req.VMIDs)}})
}

// BulkPowerOff godoc
// @Summary      Bulk power off VMs
// @Tags         vms
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  bulkActionRequest  true  "VM IDs"
// @Success      202  {object}  APIResponse
// @Router       /vms/bulk/poweroff [post]
func (h *VMHandler) BulkPowerOff(c *gin.Context) {
	var req bulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	parentTaskID, err := h.svc.BulkPowerOff(c.Request.Context(), req.VMIDs)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueChildTasks(c.Request.Context(), parentTaskID, req.VMIDs, 3)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": parentTaskID, "vm_count": len(req.VMIDs)}})
}

// BulkReboot godoc
// @Summary      Bulk reboot VMs
// @Tags         vms
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  bulkActionRequest  true  "VM IDs"
// @Success      202  {object}  APIResponse
// @Router       /vms/bulk/reboot [post]
func (h *VMHandler) BulkReboot(c *gin.Context) {
	var req bulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	parentTaskID, err := h.svc.BulkReboot(c.Request.Context(), req.VMIDs)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueChildTasks(c.Request.Context(), parentTaskID, req.VMIDs, 3)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": parentTaskID, "vm_count": len(req.VMIDs)}})
}

// BulkSnapshot godoc
// @Summary      Bulk snapshot VMs
// @Tags         vms
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  bulkSnapshotRequest  true  "VM IDs + snapshot spec"
// @Success      202  {object}  APIResponse
// @Router       /vms/bulk/snapshot [post]
func (h *VMHandler) BulkSnapshot(c *gin.Context) {
	var req bulkSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	parentTaskID, err := h.svc.BulkSnapshot(c.Request.Context(), req.VMIDs, port.SnapshotSpec{
		Name:        req.Name,
		Description: req.Description,
		Memory:      req.Memory,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	h.enqueueChildTasks(c.Request.Context(), parentTaskID, req.VMIDs, 5)
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": parentTaskID, "vm_count": len(req.VMIDs)}})
}

// enqueueChildTasks looks up child tasks for a parent and enqueues them into Redis.
// This is best-effort — the DB fallback poller will recover any that are missed.
func (h *VMHandler) enqueueChildTasks(ctx context.Context, parentTaskID string, vmIDs []string, priority int) {
	if h.enqueue == nil {
		return
	}
	// Enqueue the parent task itself so the bulk handler can track completion.
	if err := h.enqueue(ctx, parentTaskID, priority); err != nil {
		h.log.Warn("failed to enqueue parent bulk task",
			logger.String("task_id", parentTaskID),
			logger.Error(err),
		)
	}
	// Child tasks are picked up by the DB fallback poller since we don't have
	// their IDs here. For immediate dispatch, the task engine's poller interval
	// (default 5s) is acceptable for bulk operations.
}
