package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ScheduleHandler handles schedule REST endpoints.
type ScheduleHandler struct {
	svc port.ScheduleService
	log logger.Logger
}

// NewScheduleHandler creates a new ScheduleHandler.
func NewScheduleHandler(svc port.ScheduleService, log logger.Logger) *ScheduleHandler {
	return &ScheduleHandler{svc: svc, log: log}
}

// ── Request types ─────────────────────────────────────────────────────────────

type createScheduleRequest struct {
	Name           string                        `json:"name"            binding:"required"`
	Description    string                        `json:"description"`
	OperationType  model.ScheduleOperationType   `json:"operation_type"  binding:"required"`
	TargetType     model.ScheduleTargetType      `json:"target_type"     binding:"required"`
	TargetIDs      []string                      `json:"target_ids"      binding:"required,min=1"`
	ScheduleType   model.ScheduleType            `json:"schedule_type"   binding:"required"`
	CronExpression string                        `json:"cron_expression"`
	Timezone       string                        `json:"timezone"`
	Enabled        bool                          `json:"enabled"`
	MaxRuns        int                           `json:"max_runs"`
	ExpiresAt      *time.Time                    `json:"expires_at"`
	Payload        model.JSONMap                 `json:"payload"`
}

type updateScheduleRequest struct {
	Name           *string                       `json:"name"`
	Description    *string                       `json:"description"`
	OperationType  *model.ScheduleOperationType  `json:"operation_type"`
	TargetType     *model.ScheduleTargetType     `json:"target_type"`
	TargetIDs      []string                      `json:"target_ids"`
	ScheduleType   *model.ScheduleType           `json:"schedule_type"`
	CronExpression *string                       `json:"cron_expression"`
	Timezone       *string                       `json:"timezone"`
	Enabled        *bool                         `json:"enabled"`
	MaxRuns        *int                          `json:"max_runs"`
	ExpiresAt      *time.Time                    `json:"expires_at"`
	Payload        model.JSONMap                 `json:"payload"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *ScheduleHandler) List(c *gin.Context) {
	enabled := parseBoolQuery(c, "enabled")
	filter := port.ScheduleFilter{
		OperationType: c.Query("operation_type"),
		TargetType:    c.Query("target_type"),
		Enabled:       enabled,
		Status:        c.Query("status"),
	}
	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

func (h *ScheduleHandler) Create(c *gin.Context) {
	var req createScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	sched, err := h.svc.Create(c.Request.Context(), port.CreateScheduleRequest{
		Name:           req.Name,
		Description:    req.Description,
		OperationType:  req.OperationType,
		TargetType:     req.TargetType,
		TargetIDs:      req.TargetIDs,
		ScheduleType:   req.ScheduleType,
		CronExpression: req.CronExpression,
		Timezone:       req.Timezone,
		Enabled:        req.Enabled,
		MaxRuns:        req.MaxRuns,
		ExpiresAt:      req.ExpiresAt,
		Payload:        req.Payload,
	})
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	created(c, sched)
}

func (h *ScheduleHandler) GetByID(c *gin.Context) {
	sched, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, sched)
}

func (h *ScheduleHandler) Update(c *gin.Context) {
	var req updateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	sched, err := h.svc.Update(c.Request.Context(), c.Param("id"), port.UpdateScheduleRequest{
		Name:           req.Name,
		Description:    req.Description,
		OperationType:  req.OperationType,
		TargetType:     req.TargetType,
		TargetIDs:      req.TargetIDs,
		ScheduleType:   req.ScheduleType,
		CronExpression: req.CronExpression,
		Timezone:       req.Timezone,
		Enabled:        req.Enabled,
		MaxRuns:        req.MaxRuns,
		ExpiresAt:      req.ExpiresAt,
		Payload:        req.Payload,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, sched)
}

func (h *ScheduleHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

func (h *ScheduleHandler) Enable(c *gin.Context) {
	if err := h.svc.Enable(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"enabled": true})
}

func (h *ScheduleHandler) Disable(c *gin.Context) {
	if err := h.svc.Disable(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"enabled": false})
}

func (h *ScheduleHandler) TriggerNow(c *gin.Context) {
	taskID, err := h.svc.TriggerNow(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

func (h *ScheduleHandler) ListExecutions(c *gin.Context) {
	result, err := h.svc.ListExecutions(c.Request.Context(), c.Param("id"), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// parseBoolQuery parses an optional boolean query parameter.
func parseBoolQuery(c *gin.Context, key string) *bool {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	b := v == "true" || v == "1"
	return &b
}
