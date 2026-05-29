package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vmOrbit/backend/internal/api/middleware"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ApprovalHandler handles approval workflow REST endpoints.
type ApprovalHandler struct {
	svc port.ApprovalService
	log logger.Logger
}

// NewApprovalHandler creates a new ApprovalHandler.
func NewApprovalHandler(svc port.ApprovalService, log logger.Logger) *ApprovalHandler {
	return &ApprovalHandler{svc: svc, log: log}
}

// ── Request types ─────────────────────────────────────────────────────────────

type approvalDecisionRequest struct {
	Comment string `json:"comment"`
}

type escalateApprovalRequest struct {
	EscalateTo string `json:"escalate_to" binding:"required"`
	Comment    string `json:"comment"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *ApprovalHandler) List(c *gin.Context) {
	filter := port.ApprovalFilter{
		Status:       c.Query("status"),
		Operation:    c.Query("operation"),
		ResourceType: c.Query("resource_type"),
		ResourceID:   c.Query("resource_id"),
	}
	if requesterID := c.Query("requester_id"); requesterID != "" {
		if uid, err := uuid.Parse(requesterID); err == nil {
			filter.RequesterID = &uid
		}
	}
	if policyID := c.Query("policy_id"); policyID != "" {
		if uid, err := uuid.Parse(policyID); err == nil {
			filter.PolicyID = &uid
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

	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

func (h *ApprovalHandler) GetByID(c *gin.Context) {
	ar, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if isNotFound(err) {
			notFound(c, "approval request not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, ar)
}

func (h *ApprovalHandler) Approve(c *gin.Context) {
	var req approvalDecisionRequest
	_ = c.ShouldBindJSON(&req) // comment is optional

	claims := middleware.GetCurrentClaims(c)
	if claims == nil {
		unauthorized(c, "not authenticated")
		return
	}

	if err := h.svc.Approve(c.Request.Context(), c.Param("id"), port.ApprovalDecisionRequest{
		ActorID:   claims.UserID,
		ActorName: claims.Username,
		Comment:   req.Comment,
	}); err != nil {
		if isNotFound(err) {
			notFound(c, "approval request not found")
			return
		}
		badRequest(c, err.Error())
		return
	}
	ok(c, gin.H{"status": "approved"})
}

func (h *ApprovalHandler) Reject(c *gin.Context) {
	var req approvalDecisionRequest
	_ = c.ShouldBindJSON(&req)

	claims := middleware.GetCurrentClaims(c)
	if claims == nil {
		unauthorized(c, "not authenticated")
		return
	}

	if err := h.svc.Reject(c.Request.Context(), c.Param("id"), port.ApprovalDecisionRequest{
		ActorID:   claims.UserID,
		ActorName: claims.Username,
		Comment:   req.Comment,
	}); err != nil {
		if isNotFound(err) {
			notFound(c, "approval request not found")
			return
		}
		badRequest(c, err.Error())
		return
	}
	ok(c, gin.H{"status": "rejected"})
}

func (h *ApprovalHandler) Cancel(c *gin.Context) {
	claims := middleware.GetCurrentClaims(c)
	if claims == nil {
		unauthorized(c, "not authenticated")
		return
	}

	if err := h.svc.Cancel(c.Request.Context(), c.Param("id"), claims.UserID); err != nil {
		if isNotFound(err) {
			notFound(c, "approval request not found")
			return
		}
		badRequest(c, err.Error())
		return
	}
	ok(c, gin.H{"status": "cancelled"})
}

func (h *ApprovalHandler) Escalate(c *gin.Context) {
	var req escalateApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	claims := middleware.GetCurrentClaims(c)
	if claims == nil {
		unauthorized(c, "not authenticated")
		return
	}

	if err := h.svc.Escalate(c.Request.Context(), c.Param("id"), port.EscalateApprovalRequest{
		ActorID:    claims.UserID,
		ActorName:  claims.Username,
		EscalateTo: req.EscalateTo,
		Comment:    req.Comment,
	}); err != nil {
		if isNotFound(err) {
			notFound(c, "approval request not found")
			return
		}
		badRequest(c, err.Error())
		return
	}
	ok(c, gin.H{"status": "escalated"})
}

// GetPendingForMe returns approval requests pending the current user's action.
func (h *ApprovalHandler) GetPendingForMe(c *gin.Context) {
	claims := middleware.GetCurrentClaims(c)
	if claims == nil {
		unauthorized(c, "not authenticated")
		return
	}

	result, err := h.svc.GetPendingForUser(c.Request.Context(), claims.UserID, claims.Roles, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}
