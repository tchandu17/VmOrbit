package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vmOrbit/backend/internal/api/middleware"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// PolicyHandler handles policy REST endpoints.
type PolicyHandler struct {
	svc port.PolicyService
	log logger.Logger
}

// NewPolicyHandler creates a new PolicyHandler.
func NewPolicyHandler(svc port.PolicyService, log logger.Logger) *PolicyHandler {
	return &PolicyHandler{svc: svc, log: log}
}

// ── Request types ─────────────────────────────────────────────────────────────

type createPolicyRequest struct {
	Name        string                          `json:"name"        binding:"required"`
	Description string                          `json:"description"`
	Type        model.PolicyType                `json:"type"        binding:"required"`
	Effect      model.PolicyEffect              `json:"effect"      binding:"required"`
	Priority    int                             `json:"priority"`
	Enabled     bool                            `json:"enabled"`
	Operations  []string                        `json:"operations"`
	Conditions  []createPolicyConditionRequest  `json:"conditions"`
	ApprovalConfig model.JSONMap               `json:"approval_config"`
	Metadata    model.JSONMap                   `json:"metadata"`
}

type createPolicyConditionRequest struct {
	Type     model.PolicyConditionType     `json:"type"     binding:"required"`
	Operator model.PolicyConditionOperator `json:"operator" binding:"required"`
	Value    string                        `json:"value"    binding:"required"`
	Negate   bool                          `json:"negate"`
}

type updatePolicyRequest struct {
	Name        *string                         `json:"name"`
	Description *string                         `json:"description"`
	Effect      *model.PolicyEffect             `json:"effect"`
	Priority    *int                            `json:"priority"`
	Enabled     *bool                           `json:"enabled"`
	Operations  []string                        `json:"operations"`
	Conditions  []createPolicyConditionRequest  `json:"conditions"`
	ApprovalConfig model.JSONMap               `json:"approval_config"`
	Metadata    model.JSONMap                   `json:"metadata"`
}

type assignPolicyRequest struct {
	TargetType model.PolicyAssignmentTargetType `json:"target_type" binding:"required"`
	TargetID   string                           `json:"target_id"`
}

type evaluatePolicyRequest struct {
	Operation    string       `json:"operation"     binding:"required"`
	ResourceType string       `json:"resource_type" binding:"required"`
	ResourceID   string       `json:"resource_id"`
	ResourceName string       `json:"resource_name"`
	VMTags       []string     `json:"vm_tags"`
	ProviderType string       `json:"provider_type"`
	HypervisorID string       `json:"hypervisor_id"`
	Environment  string       `json:"environment"`
	BulkSize     int          `json:"bulk_size"`
	Metadata     model.JSONMap `json:"metadata"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *PolicyHandler) List(c *gin.Context) {
	filter := port.PolicyFilter{
		Type:   c.Query("type"),
		Effect: c.Query("effect"),
		Search: c.Query("search"),
	}
	if enabled := c.Query("enabled"); enabled != "" {
		b := enabled == "true"
		filter.Enabled = &b
	}

	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

func (h *PolicyHandler) Create(c *gin.Context) {
	var req createPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	conditions := make([]port.CreatePolicyConditionRequest, 0, len(req.Conditions))
	for _, cond := range req.Conditions {
		conditions = append(conditions, port.CreatePolicyConditionRequest{
			Type:     cond.Type,
			Operator: cond.Operator,
			Value:    cond.Value,
			Negate:   cond.Negate,
		})
	}

	p, err := h.svc.Create(c.Request.Context(), port.CreatePolicyRequest{
		Name:           req.Name,
		Description:    req.Description,
		Type:           req.Type,
		Effect:         req.Effect,
		Priority:       req.Priority,
		Enabled:        req.Enabled,
		Operations:     req.Operations,
		Conditions:     conditions,
		ApprovalConfig: req.ApprovalConfig,
		Metadata:       req.Metadata,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, p)
}

func (h *PolicyHandler) GetByID(c *gin.Context) {
	p, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if isNotFound(err) {
			notFound(c, "policy not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, p)
}

func (h *PolicyHandler) Update(c *gin.Context) {
	var req updatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	var conditions []port.CreatePolicyConditionRequest
	if req.Conditions != nil {
		conditions = make([]port.CreatePolicyConditionRequest, 0, len(req.Conditions))
		for _, cond := range req.Conditions {
			conditions = append(conditions, port.CreatePolicyConditionRequest{
				Type:     cond.Type,
				Operator: cond.Operator,
				Value:    cond.Value,
				Negate:   cond.Negate,
			})
		}
	}

	p, err := h.svc.Update(c.Request.Context(), c.Param("id"), port.UpdatePolicyRequest{
		Name:           req.Name,
		Description:    req.Description,
		Effect:         req.Effect,
		Priority:       req.Priority,
		Enabled:        req.Enabled,
		Operations:     req.Operations,
		Conditions:     conditions,
		ApprovalConfig: req.ApprovalConfig,
		Metadata:       req.Metadata,
	})
	if err != nil {
		if isNotFound(err) {
			notFound(c, "policy not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, p)
}

func (h *PolicyHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if isNotFound(err) {
			notFound(c, "policy not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

func (h *PolicyHandler) Enable(c *gin.Context) {
	if err := h.svc.Enable(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"enabled": true})
}

func (h *PolicyHandler) Disable(c *gin.Context) {
	if err := h.svc.Disable(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"enabled": false})
}

func (h *PolicyHandler) Assign(c *gin.Context) {
	var req assignPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	assignment, err := h.svc.Assign(c.Request.Context(), c.Param("id"), port.AssignPolicyRequest{
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
	})
	if err != nil {
		if isNotFound(err) {
			notFound(c, "policy not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	created(c, assignment)
}

func (h *PolicyHandler) Unassign(c *gin.Context) {
	if err := h.svc.Unassign(c.Request.Context(), c.Param("assignmentId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

func (h *PolicyHandler) ListAssignments(c *gin.Context) {
	assignments, err := h.svc.ListAssignments(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, assignments)
}

func (h *PolicyHandler) ListViolations(c *gin.Context) {
	filter := port.PolicyViolationFilter{
		Operation:    c.Query("operation"),
		ResourceType: c.Query("resource_type"),
		ResourceID:   c.Query("resource_id"),
		Status:       c.Query("status"),
	}
	if policyID := c.Query("policy_id"); policyID != "" {
		if uid, err := uuid.Parse(policyID); err == nil {
			filter.PolicyID = &uid
		}
	}
	if userID := c.Query("user_id"); userID != "" {
		if uid, err := uuid.Parse(userID); err == nil {
			filter.UserID = &uid
		}
	}
	if since := c.Query("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := c.Query("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}

	result, err := h.svc.ListViolations(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

func (h *PolicyHandler) Evaluate(c *gin.Context) {
	var req evaluatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	claims := middleware.GetCurrentClaims(c)
	userID := ""
	username := ""
	var roles []string
	if claims != nil {
		userID = claims.UserID
		username = claims.Username
		roles = claims.Roles
	}

	result, err := h.svc.Evaluate(c.Request.Context(), port.PolicyEvaluationRequest{
		Operation:    req.Operation,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		UserID:       userID,
		Username:     username,
		Roles:        roles,
		VMTags:       req.VMTags,
		ProviderType: req.ProviderType,
		HypervisorID: req.HypervisorID,
		Environment:  req.Environment,
		BulkSize:     req.BulkSize,
		Metadata:     req.Metadata,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, result)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

