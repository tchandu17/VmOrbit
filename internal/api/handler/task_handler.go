package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// TaskHandler handles async task REST endpoints.
type TaskHandler struct {
	svc port.TaskService
	log logger.Logger
}

// NewTaskHandler creates a new TaskHandler.
func NewTaskHandler(svc port.TaskService, log logger.Logger) *TaskHandler {
	return &TaskHandler{svc: svc, log: log}
}

// List godoc
// @Summary      List tasks
// @Tags         tasks
// @Security     BearerAuth
// @Produce      json
// @Param        page       query  int  false  "Page number"
// @Param        page_size  query  int  false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /tasks [get]
func (h *TaskHandler) List(c *gin.Context) {
	result, err := h.svc.List(c.Request.Context(), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// GetByID godoc
// @Summary      Get task by ID
// @Tags         tasks
// @Security     BearerAuth
// @Param        id  path      string  true  "Task ID"
// @Success      200  {object}  APIResponse
// @Router       /tasks/{id} [get]
func (h *TaskHandler) GetByID(c *gin.Context) {
	task, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, task)
}

// Cancel godoc
// @Summary      Cancel a pending or running task
// @Tags         tasks
// @Security     BearerAuth
// @Param        id  path  string  true  "Task ID"
// @Success      204
// @Router       /tasks/{id} [delete]
func (h *TaskHandler) Cancel(c *gin.Context) {
	if err := h.svc.Cancel(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// GetLogs godoc
// @Summary      Stream task logs (paginated)
// @Tags         tasks
// @Security     BearerAuth
// @Param        id         path   string  true   "Task ID"
// @Param        page       query  int     false  "Page number"
// @Param        page_size  query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /tasks/{id}/logs [get]
func (h *TaskHandler) GetLogs(c *gin.Context) {
	result, err := h.svc.GetLogs(c.Request.Context(), c.Param("id"), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}
