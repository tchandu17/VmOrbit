package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// approvalService implements port.ApprovalService.
type approvalService struct {
	requestRepo port.ApprovalRequestRepository
	stepRepo    port.ApprovalStepRepository
	historyRepo port.ApprovalHistoryRepository
	audit       port.AuditService
	log         logger.Logger
}

// NewApprovalService creates a new approval service.
func NewApprovalService(
	requestRepo port.ApprovalRequestRepository,
	stepRepo port.ApprovalStepRepository,
	historyRepo port.ApprovalHistoryRepository,
	audit port.AuditService,
	log logger.Logger,
) port.ApprovalService {
	return &approvalService{
		requestRepo: requestRepo,
		stepRepo:    stepRepo,
		historyRepo: historyRepo,
		audit:       audit,
		log:         log,
	}
}

func (s *approvalService) Create(ctx context.Context, req port.CreateApprovalRequest) (*model.ApprovalRequest, error) {
	policyUUID, err := uuid.Parse(req.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("invalid policy ID: %w", err)
	}

	requesterUUID, err := uuid.Parse(req.RequesterID)
	if err != nil {
		return nil, fmt.Errorf("invalid requester ID: %w", err)
	}

	now := time.Now().UTC()
	ar := &model.ApprovalRequest{
		PolicyID:         policyUUID,
		PolicyName:       req.PolicyName,
		Operation:        req.Operation,
		ResourceType:     req.ResourceType,
		ResourceID:       req.ResourceID,
		ResourceName:     req.ResourceName,
		RequesterID:      requesterUUID,
		RequesterName:    req.RequesterName,
		Justification:    req.Justification,
		Status:           model.ApprovalStatusPending,
		OperationPayload: req.OperationPayload,
		Metadata:         req.Metadata,
	}

	if req.ExpiresIn > 0 {
		expiresAt := now.Add(req.ExpiresIn)
		ar.ExpiresAt = &expiresAt
	}

	if err := s.requestRepo.Create(ctx, ar); err != nil {
		return nil, fmt.Errorf("creating approval request: %w", err)
	}

	// Create approval steps
	if len(req.Steps) > 0 {
		for _, stepReq := range req.Steps {
			step := &model.ApprovalStep{
				RequestID:    ar.ID,
				StepOrder:    stepReq.StepOrder,
				Status:       model.ApprovalStepStatusPending,
				ApproverName: stepReq.ApproverName,
				ApproverRole: stepReq.ApproverRole,
			}
			if stepReq.ApproverID != "" {
				if uid, err := uuid.Parse(stepReq.ApproverID); err == nil {
					step.ApproverID = &uid
				}
			}
			if err := s.stepRepo.Create(ctx, step); err != nil {
				s.log.Warn("failed to create approval step",
					logger.Error(err),
					logger.String("request_id", ar.ID.String()),
				)
			}
		}
	} else {
		// Create a default single-step requiring any admin
		step := &model.ApprovalStep{
			RequestID:    ar.ID,
			StepOrder:    1,
			Status:       model.ApprovalStepStatusPending,
			ApproverRole: "admin",
			ApproverName: "Administrator",
		}
		if err := s.stepRepo.Create(ctx, step); err != nil {
			s.log.Warn("failed to create default approval step", logger.Error(err))
		}
	}

	// Record creation in history
	s.appendHistory(ctx, ar.ID, model.ApprovalHistoryActionCreated, req.RequesterID, req.RequesterName, "Approval request created", nil)

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionCreate,
		Resource:    "approval_request",
		ResourceID:  ar.ID.String(),
		Description: fmt.Sprintf("approval request created for %s on %s %s", req.Operation, req.ResourceType, req.ResourceName),
		Success:     true,
	})

	return ar, nil
}

func (s *approvalService) GetByID(ctx context.Context, id string) (*model.ApprovalRequest, error) {
	return s.requestRepo.GetByID(ctx, id)
}

func (s *approvalService) List(ctx context.Context, filter port.ApprovalFilter, page port.Page) (*port.PageResult[model.ApprovalRequest], error) {
	return s.requestRepo.List(ctx, filter, page)
}

func (s *approvalService) Approve(ctx context.Context, id string, req port.ApprovalDecisionRequest) error {
	ar, err := s.requestRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("approval request not found: %w", err)
	}

	if ar.Status != model.ApprovalStatusPending {
		return fmt.Errorf("approval request is not pending (status: %s)", ar.Status)
	}

	// Get the current pending step
	step, err := s.stepRepo.GetCurrentPendingStep(ctx, id)
	if err != nil {
		return fmt.Errorf("no pending step found: %w", err)
	}

	// Verify the actor can approve this step
	if err := s.verifyApprover(step, req.ActorID); err != nil {
		return err
	}

	// Mark step as approved
	now := time.Now().UTC()
	step.Status = model.ApprovalStepStatusApproved
	step.ResolvedAt = &now
	step.Comment = req.Comment
	step.ResolvedByName = req.ActorName
	if uid, err := uuid.Parse(req.ActorID); err == nil {
		step.ResolvedBy = &uid
	}
	if err := s.stepRepo.Update(ctx, step); err != nil {
		return fmt.Errorf("updating step: %w", err)
	}

	// Check if there are more pending steps
	nextStep, err := s.stepRepo.GetCurrentPendingStep(ctx, id)
	if err != nil || nextStep == nil {
		// All steps approved — mark request as approved
		ar.Status = model.ApprovalStatusApproved
		ar.ResolvedAt = &now
		if err := s.requestRepo.Update(ctx, ar); err != nil {
			return fmt.Errorf("updating approval request: %w", err)
		}
		s.appendHistory(ctx, ar.ID, model.ApprovalHistoryActionApproved, req.ActorID, req.ActorName, req.Comment, nil)
		s.log.Info("approval request approved",
			logger.String("request_id", id),
			logger.String("actor", req.ActorName),
		)
	} else {
		// More steps remain — record partial approval
		s.appendHistory(ctx, ar.ID, model.ApprovalHistoryActionApproved,
			req.ActorID, req.ActorName,
			fmt.Sprintf("Step %d approved: %s", step.StepOrder, req.Comment), nil)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "approval_request",
		ResourceID:  id,
		Description: fmt.Sprintf("approval request approved by %s", req.ActorName),
		Success:     true,
	})

	return nil
}

func (s *approvalService) Reject(ctx context.Context, id string, req port.ApprovalDecisionRequest) error {
	ar, err := s.requestRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("approval request not found: %w", err)
	}

	if ar.Status != model.ApprovalStatusPending {
		return fmt.Errorf("approval request is not pending (status: %s)", ar.Status)
	}

	// Get the current pending step
	step, err := s.stepRepo.GetCurrentPendingStep(ctx, id)
	if err != nil {
		return fmt.Errorf("no pending step found: %w", err)
	}

	// Verify the actor can reject this step
	if err := s.verifyApprover(step, req.ActorID); err != nil {
		return err
	}

	// Mark step as rejected
	now := time.Now().UTC()
	step.Status = model.ApprovalStepStatusRejected
	step.ResolvedAt = &now
	step.Comment = req.Comment
	step.ResolvedByName = req.ActorName
	if uid, err := uuid.Parse(req.ActorID); err == nil {
		step.ResolvedBy = &uid
	}
	if err := s.stepRepo.Update(ctx, step); err != nil {
		return fmt.Errorf("updating step: %w", err)
	}

	// Mark request as rejected
	ar.Status = model.ApprovalStatusRejected
	ar.ResolvedAt = &now
	if err := s.requestRepo.Update(ctx, ar); err != nil {
		return fmt.Errorf("updating approval request: %w", err)
	}

	s.appendHistory(ctx, ar.ID, model.ApprovalHistoryActionRejected, req.ActorID, req.ActorName, req.Comment, nil)

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "approval_request",
		ResourceID:  id,
		Description: fmt.Sprintf("approval request rejected by %s: %s", req.ActorName, req.Comment),
		Success:     true,
	})

	return nil
}

func (s *approvalService) Cancel(ctx context.Context, id string, userID string) error {
	ar, err := s.requestRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("approval request not found: %w", err)
	}

	if ar.Status != model.ApprovalStatusPending {
		return fmt.Errorf("only pending requests can be cancelled")
	}

	// Only the requester can cancel
	if ar.RequesterID.String() != userID {
		return fmt.Errorf("only the requester can cancel this approval request")
	}

	now := time.Now().UTC()
	ar.Status = model.ApprovalStatusCancelled
	ar.ResolvedAt = &now
	if err := s.requestRepo.Update(ctx, ar); err != nil {
		return fmt.Errorf("cancelling approval request: %w", err)
	}

	s.appendHistory(ctx, ar.ID, model.ApprovalHistoryActionCancelled, userID, ar.RequesterName, "Cancelled by requester", nil)
	return nil
}

func (s *approvalService) Escalate(ctx context.Context, id string, req port.EscalateApprovalRequest) error {
	ar, err := s.requestRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("approval request not found: %w", err)
	}

	if ar.Status != model.ApprovalStatusPending {
		return fmt.Errorf("only pending requests can be escalated")
	}

	now := time.Now().UTC()
	ar.Status = model.ApprovalStatusEscalated
	ar.EscalatedAt = &now
	if uid, err := uuid.Parse(req.EscalateTo); err == nil {
		ar.EscalatedTo = &uid
	}
	if err := s.requestRepo.Update(ctx, ar); err != nil {
		return fmt.Errorf("escalating approval request: %w", err)
	}

	s.appendHistory(ctx, ar.ID, model.ApprovalHistoryActionEscalated, req.ActorID, req.ActorName,
		fmt.Sprintf("Escalated to %s: %s", req.EscalateTo, req.Comment), nil)

	return nil
}

func (s *approvalService) ExpireStale(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	expired, err := s.requestRepo.ListExpired(ctx, now)
	if err != nil {
		return 0, err
	}

	count := 0
	for i := range expired {
		ar := &expired[i]
		ar.Status = model.ApprovalStatusExpired
		resolvedAt := now
		ar.ResolvedAt = &resolvedAt
		if err := s.requestRepo.Update(ctx, ar); err != nil {
			s.log.Warn("failed to expire approval request",
				logger.String("id", ar.ID.String()),
				logger.Error(err),
			)
			continue
		}
		s.appendHistory(ctx, ar.ID, model.ApprovalHistoryActionExpired, "", "system", "Request expired", nil)
		count++
	}

	return count, nil
}

func (s *approvalService) GetPendingForUser(ctx context.Context, userID string, roles []string, page port.Page) (*port.PageResult[model.ApprovalRequest], error) {
	return s.requestRepo.ListPendingForUser(ctx, userID, roles, page)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// verifyApprover checks that the actor is allowed to act on the given step.
// Super-admins bypass this check (handled at the handler level via RBAC).
func (s *approvalService) verifyApprover(step *model.ApprovalStep, actorID string) error {
	// If step has a specific approver, verify it matches
	if step.ApproverID != nil && step.ApproverID.String() != actorID {
		return fmt.Errorf("you are not the designated approver for this step")
	}
	// Role-based approval is verified at the handler level via JWT claims
	return nil
}

func (s *approvalService) appendHistory(
	ctx context.Context,
	requestID uuid.UUID,
	action model.ApprovalHistoryAction,
	actorID, actorName, comment string,
	metadata model.JSONMap,
) {
	h := &model.ApprovalHistory{
		RequestID: requestID,
		Action:    action,
		ActorName: actorName,
		Comment:   comment,
		Metadata:  metadata,
	}
	if actorID != "" {
		if uid, err := uuid.Parse(actorID); err == nil {
			h.ActorID = &uid
		}
	}
	if err := s.historyRepo.Create(ctx, h); err != nil {
		s.log.Warn("failed to append approval history",
			logger.Error(err),
			logger.String("request_id", requestID.String()),
		)
	}
}
