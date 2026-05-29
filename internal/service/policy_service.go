package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/policy"
	"github.com/vmOrbit/backend/pkg/logger"
)

// policyService implements port.PolicyService.
type policyService struct {
	repo           port.PolicyRepository
	assignmentRepo port.PolicyAssignmentRepository
	violationRepo  port.PolicyViolationRepository
	engine         *policy.Engine
	audit          port.AuditService
	log            logger.Logger
}

// NewPolicyService creates a new policy service.
func NewPolicyService(
	repo port.PolicyRepository,
	assignmentRepo port.PolicyAssignmentRepository,
	violationRepo port.PolicyViolationRepository,
	engine *policy.Engine,
	audit port.AuditService,
	log logger.Logger,
) port.PolicyService {
	return &policyService{
		repo:           repo,
		assignmentRepo: assignmentRepo,
		violationRepo:  violationRepo,
		engine:         engine,
		audit:          audit,
		log:            log,
	}
}

func (s *policyService) Create(ctx context.Context, req port.CreatePolicyRequest) (*model.Policy, error) {
	p := &model.Policy{
		Name:           req.Name,
		Description:    req.Description,
		Type:           req.Type,
		Effect:         req.Effect,
		Priority:       req.Priority,
		Enabled:        req.Enabled,
		Operations:     req.Operations,
		ApprovalConfig: req.ApprovalConfig,
		Metadata:       req.Metadata,
	}
	if p.Priority == 0 {
		p.Priority = 100
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("creating policy: %w", err)
	}

	// Create conditions
	if len(req.Conditions) > 0 {
		conditions := make([]model.PolicyCondition, 0, len(req.Conditions))
		for _, c := range req.Conditions {
			conditions = append(conditions, model.PolicyCondition{
				PolicyID: p.ID,
				Type:     c.Type,
				Operator: c.Operator,
				Value:    c.Value,
				Negate:   c.Negate,
			})
		}
		if err := s.repo.ReplaceConditions(ctx, p.ID.String(), conditions); err != nil {
			return nil, fmt.Errorf("creating policy conditions: %w", err)
		}
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionCreate,
		Resource:    "policy",
		ResourceID:  p.ID.String(),
		Description: fmt.Sprintf("created policy %q", p.Name),
		Success:     true,
	})

	return s.repo.GetByID(ctx, p.ID.String())
}

func (s *policyService) GetByID(ctx context.Context, id string) (*model.Policy, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *policyService) Update(ctx context.Context, id string, req port.UpdatePolicyRequest) (*model.Policy, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Effect != nil {
		p.Effect = *req.Effect
	}
	if req.Priority != nil {
		p.Priority = *req.Priority
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	if req.Operations != nil {
		p.Operations = req.Operations
	}
	if req.ApprovalConfig != nil {
		p.ApprovalConfig = req.ApprovalConfig
	}
	if req.Metadata != nil {
		p.Metadata = req.Metadata
	}

	if err := s.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("updating policy: %w", err)
	}

	// Replace conditions if provided
	if req.Conditions != nil {
		conditions := make([]model.PolicyCondition, 0, len(req.Conditions))
		policyUUID, _ := uuid.Parse(id)
		for _, c := range req.Conditions {
			conditions = append(conditions, model.PolicyCondition{
				PolicyID: policyUUID,
				Type:     c.Type,
				Operator: c.Operator,
				Value:    c.Value,
				Negate:   c.Negate,
			})
		}
		if err := s.repo.ReplaceConditions(ctx, id, conditions); err != nil {
			return nil, fmt.Errorf("updating policy conditions: %w", err)
		}
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionUpdate,
		Resource:   "policy",
		ResourceID: id,
		Success:    true,
	})

	return s.repo.GetByID(ctx, id)
}

func (s *policyService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionDelete,
		Resource:   "policy",
		ResourceID: id,
		Success:    true,
	})
	return nil
}

func (s *policyService) List(ctx context.Context, filter port.PolicyFilter, page port.Page) (*port.PageResult[model.Policy], error) {
	return s.repo.List(ctx, filter, page)
}

func (s *policyService) Enable(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	p.Enabled = true
	return s.repo.Update(ctx, p)
}

func (s *policyService) Disable(ctx context.Context, id string) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	p.Enabled = false
	return s.repo.Update(ctx, p)
}

func (s *policyService) Assign(ctx context.Context, policyID string, req port.AssignPolicyRequest) (*model.PolicyAssignment, error) {
	policyUUID, err := uuid.Parse(policyID)
	if err != nil {
		return nil, fmt.Errorf("invalid policy ID: %w", err)
	}

	// Verify policy exists
	if _, err := s.repo.GetByID(ctx, policyID); err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}

	a := &model.PolicyAssignment{
		PolicyID:   policyUUID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		CreatedBy:  callerUUID(ctx),
	}

	if err := s.assignmentRepo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("creating assignment: %w", err)
	}

	return a, nil
}

func (s *policyService) Unassign(ctx context.Context, assignmentID string) error {
	return s.assignmentRepo.Delete(ctx, assignmentID)
}

func (s *policyService) ListAssignments(ctx context.Context, policyID string) ([]model.PolicyAssignment, error) {
	return s.assignmentRepo.ListByPolicy(ctx, policyID)
}

func (s *policyService) ListViolations(ctx context.Context, filter port.PolicyViolationFilter, page port.Page) (*port.PageResult[model.PolicyViolation], error) {
	return s.violationRepo.List(ctx, filter, page)
}

func (s *policyService) Evaluate(ctx context.Context, req port.PolicyEvaluationRequest) (*port.PolicyEvaluationResult, error) {
	return s.engine.Evaluate(ctx, req)
}
