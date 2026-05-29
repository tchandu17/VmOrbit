package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// WorkflowHandler handles automation workflow REST endpoints.
type WorkflowHandler struct {
	svc port.WorkflowService
	log logger.Logger
}

// NewWorkflowHandler creates a new WorkflowHandler.
func NewWorkflowHandler(svc port.WorkflowService, log logger.Logger) *WorkflowHandler {
	return &WorkflowHandler{svc: svc, log: log}
}

// ── Request types ─────────────────────────────────────────────────────────────

type workflowActionRequest struct {
	Order           int                        `json:"order"`
	ActionType      model.WorkflowActionType   `json:"action_type"      binding:"required"`
	Name            string                     `json:"name"`
	Description     string                     `json:"description"`
	Config          model.JSONMap              `json:"config"`
	RetryCount      int                        `json:"retry_count"`
	TimeoutSeconds  int                        `json:"timeout_seconds"`
	ContinueOnError *bool                      `json:"continue_on_error"`
}

type createWorkflowRequest struct {
	Name              string                        `json:"name"         binding:"required"`
	Description       string                        `json:"description"`
	Enabled           bool                          `json:"enabled"`
	TriggerType       model.WorkflowTriggerType     `json:"trigger_type" binding:"required"`
	TriggerConfig     model.JSONMap                 `json:"trigger_config"`
	Conditions        model.JSONMap                 `json:"conditions"`
	ContinueOnError   bool                          `json:"continue_on_error"`
	MaxConcurrentRuns int                           `json:"max_concurrent_runs"`
	Actions           []workflowActionRequest       `json:"actions"`
}

type updateWorkflowRequest struct {
	Name              *string                       `json:"name"`
	Description       *string                       `json:"description"`
	Enabled           *bool                         `json:"enabled"`
	TriggerType       *model.WorkflowTriggerType    `json:"trigger_type"`
	TriggerConfig     model.JSONMap                 `json:"trigger_config"`
	Conditions        model.JSONMap                 `json:"conditions"`
	ContinueOnError   *bool                         `json:"continue_on_error"`
	MaxConcurrentRuns *int                          `json:"max_concurrent_runs"`
	Actions           []workflowActionRequest       `json:"actions"`
}

type triggerWorkflowRequest struct {
	TriggerData model.JSONMap `json:"trigger_data"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *WorkflowHandler) List(c *gin.Context) {
	enabled := parseBoolQuery(c, "enabled")
	filter := port.WorkflowFilter{
		TriggerType: c.Query("trigger_type"),
		Enabled:     enabled,
		Status:      c.Query("status"),
	}
	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

func (h *WorkflowHandler) Create(c *gin.Context) {
	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	actions := make([]port.CreateWorkflowActionRequest, len(req.Actions))
	for i, a := range req.Actions {
		actions[i] = port.CreateWorkflowActionRequest{
			Order:           a.Order,
			ActionType:      a.ActionType,
			Name:            a.Name,
			Description:     a.Description,
			Config:          a.Config,
			RetryCount:      a.RetryCount,
			TimeoutSeconds:  a.TimeoutSeconds,
			ContinueOnError: a.ContinueOnError,
		}
	}

	w, err := h.svc.Create(c.Request.Context(), port.CreateWorkflowRequest{
		Name:              req.Name,
		Description:       req.Description,
		Enabled:           req.Enabled,
		TriggerType:       req.TriggerType,
		TriggerConfig:     req.TriggerConfig,
		Conditions:        req.Conditions,
		ContinueOnError:   req.ContinueOnError,
		MaxConcurrentRuns: req.MaxConcurrentRuns,
		Actions:           actions,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, w)
}

func (h *WorkflowHandler) GetByID(c *gin.Context) {
	w, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, w)
}

func (h *WorkflowHandler) Update(c *gin.Context) {
	var req updateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	var actions []port.CreateWorkflowActionRequest
	if req.Actions != nil {
		actions = make([]port.CreateWorkflowActionRequest, len(req.Actions))
		for i, a := range req.Actions {
			actions[i] = port.CreateWorkflowActionRequest{
				Order:           a.Order,
				ActionType:      a.ActionType,
				Name:            a.Name,
				Description:     a.Description,
				Config:          a.Config,
				RetryCount:      a.RetryCount,
				TimeoutSeconds:  a.TimeoutSeconds,
				ContinueOnError: a.ContinueOnError,
			}
		}
	}

	w, err := h.svc.Update(c.Request.Context(), c.Param("id"), port.UpdateWorkflowRequest{
		Name:              req.Name,
		Description:       req.Description,
		Enabled:           req.Enabled,
		TriggerType:       req.TriggerType,
		TriggerConfig:     req.TriggerConfig,
		Conditions:        req.Conditions,
		ContinueOnError:   req.ContinueOnError,
		MaxConcurrentRuns: req.MaxConcurrentRuns,
		Actions:           actions,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, w)
}

func (h *WorkflowHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

func (h *WorkflowHandler) Enable(c *gin.Context) {
	if err := h.svc.Enable(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"enabled": true})
}

func (h *WorkflowHandler) Disable(c *gin.Context) {
	if err := h.svc.Disable(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"enabled": false})
}

func (h *WorkflowHandler) TriggerNow(c *gin.Context) {
	var req triggerWorkflowRequest
	_ = c.ShouldBindJSON(&req) // optional body

	runID, err := h.svc.TriggerNow(c.Request.Context(), c.Param("id"), req.TriggerData)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	c.JSON(202, APIResponse{Success: true, Data: gin.H{"run_id": runID}})
}

func (h *WorkflowHandler) ListRuns(c *gin.Context) {
	result, err := h.svc.ListRuns(c.Request.Context(), c.Param("id"), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

func (h *WorkflowHandler) GetRun(c *gin.Context) {
	run, err := h.svc.GetRun(c.Request.Context(), c.Param("runId"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, run)
}
