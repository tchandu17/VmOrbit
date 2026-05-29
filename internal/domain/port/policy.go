package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Policy service interfaces
// ─────────────────────────────────────────────────────────────────────────────

// PolicyService manages policy lifecycle and evaluation.
type PolicyService interface {
	// CRUD
	Create(ctx context.Context, req CreatePolicyRequest) (*model.Policy, error)
	GetByID(ctx context.Context, id string) (*model.Policy, error)
	Update(ctx context.Context, id string, req UpdatePolicyRequest) (*model.Policy, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter PolicyFilter, page Page) (*PageResult[model.Policy], error)
	Enable(ctx context.Context, id string) error
	Disable(ctx context.Context, id string) error

	// Assignments
	Assign(ctx context.Context, policyID string, req AssignPolicyRequest) (*model.PolicyAssignment, error)
	Unassign(ctx context.Context, assignmentID string) error
	ListAssignments(ctx context.Context, policyID string) ([]model.PolicyAssignment, error)

	// Violations
	ListViolations(ctx context.Context, filter PolicyViolationFilter, page Page) (*PageResult[model.PolicyViolation], error)

	// Evaluation — called by the policy middleware before executing operations
	Evaluate(ctx context.Context, req PolicyEvaluationRequest) (*PolicyEvaluationResult, error)
}

// CreatePolicyRequest carries new-policy data.
type CreatePolicyRequest struct {
	Name           string
	Description    string
	Type           model.PolicyType
	Effect         model.PolicyEffect
	Priority       int
	Enabled        bool
	Operations     []string
	Conditions     []CreatePolicyConditionRequest
	ApprovalConfig model.JSONMap
	Metadata       model.JSONMap
}

// CreatePolicyConditionRequest carries a single condition definition.
type CreatePolicyConditionRequest struct {
	Type     model.PolicyConditionType
	Operator model.PolicyConditionOperator
	Value    string
	Negate   bool
}

// UpdatePolicyRequest carries policy update data.
type UpdatePolicyRequest struct {
	Name           *string
	Description    *string
	Effect         *model.PolicyEffect
	Priority       *int
	Enabled        *bool
	Operations     []string
	Conditions     []CreatePolicyConditionRequest // nil = no change; non-nil = replace all
	ApprovalConfig model.JSONMap
	Metadata       model.JSONMap
}

// PolicyFilter narrows policy list queries.
type PolicyFilter struct {
	Type    string
	Effect  string
	Enabled *bool
	Search  string
}

// AssignPolicyRequest carries policy assignment data.
type AssignPolicyRequest struct {
	TargetType model.PolicyAssignmentTargetType
	TargetID   string
}

// PolicyViolationFilter narrows violation list queries.
type PolicyViolationFilter struct {
	PolicyID     *uuid.UUID
	UserID       *uuid.UUID
	Operation    string
	ResourceType string
	ResourceID   string
	Status       string
	Since        *time.Time
	Until        *time.Time
}

// PolicyEvaluationRequest carries the context for evaluating policies.
type PolicyEvaluationRequest struct {
	// The operation being attempted (e.g. "vm.power_off", "vm.delete")
	Operation string
	// Resource being operated on
	ResourceType string
	ResourceID   string
	ResourceName string
	// Caller context
	UserID   string
	Username string
	Roles    []string
	// Additional context for condition evaluation
	VMTags       []string
	ProviderType string
	HypervisorID string
	Environment  string
	BulkSize     int
	Metadata     model.JSONMap
}

// PolicyEvaluationResult is the outcome of evaluating all applicable policies.
type PolicyEvaluationResult struct {
	// Allowed is true when no deny policy matched and no approval is required.
	Allowed bool
	// Effect is the winning policy effect (empty if no policy matched).
	Effect model.PolicyEffect
	// MatchedPolicy is the policy that produced the effect (nil if none matched).
	MatchedPolicy *model.Policy
	// ViolationID is set when a violation record was created.
	ViolationID string
	// ApprovalRequestID is set when effect = require_approval.
	ApprovalRequestID string
	// Message is a human-readable explanation of the decision.
	Message string
	// RequiresSnapshot is true when effect = require_snapshot.
	RequiresSnapshot bool
	// RequiresJustification is true when effect = require_justification.
	RequiresJustification bool
}

// ─────────────────────────────────────────────────────────────────────────────
// Approval service interfaces
// ─────────────────────────────────────────────────────────────────────────────

// ApprovalService manages approval request lifecycle.
type ApprovalService interface {
	// Create creates a new approval request (called by policy engine).
	Create(ctx context.Context, req CreateApprovalRequest) (*model.ApprovalRequest, error)
	// GetByID returns an approval request with its steps and history.
	GetByID(ctx context.Context, id string) (*model.ApprovalRequest, error)
	// List returns paginated approval requests.
	List(ctx context.Context, filter ApprovalFilter, page Page) (*PageResult[model.ApprovalRequest], error)
	// Approve approves the current pending step.
	Approve(ctx context.Context, id string, req ApprovalDecisionRequest) error
	// Reject rejects the approval request.
	Reject(ctx context.Context, id string, req ApprovalDecisionRequest) error
	// Cancel cancels a pending approval request (by the requester).
	Cancel(ctx context.Context, id string, userID string) error
	// Escalate escalates the request to a higher authority.
	Escalate(ctx context.Context, id string, req EscalateApprovalRequest) error
	// ExpireStale marks requests past their expiry as expired.
	ExpireStale(ctx context.Context) (int, error)
	// GetPendingForUser returns pending requests that the user can act on.
	GetPendingForUser(ctx context.Context, userID string, roles []string, page Page) (*PageResult[model.ApprovalRequest], error)
}

// CreateApprovalRequest carries new approval request data.
type CreateApprovalRequest struct {
	PolicyID         string
	PolicyName       string
	Operation        string
	ResourceType     string
	ResourceID       string
	ResourceName     string
	RequesterID      string
	RequesterName    string
	Justification    string
	OperationPayload model.JSONMap
	Metadata         model.JSONMap
	// Steps defines the approval chain. If empty, a single global-approver step is created.
	Steps []CreateApprovalStepRequest
	// ExpiresIn is how long the request is valid. Zero = no expiry.
	ExpiresIn time.Duration
}

// CreateApprovalStepRequest carries a single step definition.
type CreateApprovalStepRequest struct {
	StepOrder    int
	ApproverID   string // specific user UUID, or empty if role-based
	ApproverRole string // role name, or empty if user-based
	ApproverName string
}

// ApprovalDecisionRequest carries approve/reject data.
type ApprovalDecisionRequest struct {
	ActorID   string
	ActorName string
	Comment   string
}

// EscalateApprovalRequest carries escalation data.
type EscalateApprovalRequest struct {
	ActorID     string
	ActorName   string
	EscalateTo  string // user UUID to escalate to
	Comment     string
}

// ApprovalFilter narrows approval list queries.
type ApprovalFilter struct {
	Status       string
	RequesterID  *uuid.UUID
	PolicyID     *uuid.UUID
	Operation    string
	ResourceType string
	ResourceID   string
	Since        *time.Time
	Until        *time.Time
}

// ─────────────────────────────────────────────────────────────────────────────
// Repository interfaces
// ─────────────────────────────────────────────────────────────────────────────

// PolicyRepository defines persistence operations for policies.
type PolicyRepository interface {
	Create(ctx context.Context, p *model.Policy) error
	GetByID(ctx context.Context, id string) (*model.Policy, error)
	Update(ctx context.Context, p *model.Policy) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter PolicyFilter, page Page) (*PageResult[model.Policy], error)
	// ListEnabled returns all enabled policies ordered by priority.
	ListEnabled(ctx context.Context) ([]model.Policy, error)
	// ReplaceConditions replaces all conditions for a policy.
	ReplaceConditions(ctx context.Context, policyID string, conditions []model.PolicyCondition) error
}

// PolicyAssignmentRepository defines persistence for policy assignments.
type PolicyAssignmentRepository interface {
	Create(ctx context.Context, a *model.PolicyAssignment) error
	GetByID(ctx context.Context, id string) (*model.PolicyAssignment, error)
	Delete(ctx context.Context, id string) error
	ListByPolicy(ctx context.Context, policyID string) ([]model.PolicyAssignment, error)
	// ListForContext returns assignments relevant to the given evaluation context.
	// Returns global assignments + assignments matching any of the provided target IDs.
	ListForContext(ctx context.Context, targetIDs []string) ([]model.PolicyAssignment, error)
}

// PolicyViolationRepository defines persistence for policy violations.
type PolicyViolationRepository interface {
	Create(ctx context.Context, v *model.PolicyViolation) error
	GetByID(ctx context.Context, id string) (*model.PolicyViolation, error)
	List(ctx context.Context, filter PolicyViolationFilter, page Page) (*PageResult[model.PolicyViolation], error)
}

// ApprovalRequestRepository defines persistence for approval requests.
type ApprovalRequestRepository interface {
	Create(ctx context.Context, r *model.ApprovalRequest) error
	GetByID(ctx context.Context, id string) (*model.ApprovalRequest, error)
	Update(ctx context.Context, r *model.ApprovalRequest) error
	List(ctx context.Context, filter ApprovalFilter, page Page) (*PageResult[model.ApprovalRequest], error)
	// ListExpired returns pending requests past their expiry time.
	ListExpired(ctx context.Context, now time.Time) ([]model.ApprovalRequest, error)
	// ListPendingForUser returns requests where the user is an approver (by ID or role).
	ListPendingForUser(ctx context.Context, userID string, roles []string, page Page) (*PageResult[model.ApprovalRequest], error)
}

// ApprovalStepRepository defines persistence for approval steps.
type ApprovalStepRepository interface {
	Create(ctx context.Context, s *model.ApprovalStep) error
	GetByID(ctx context.Context, id string) (*model.ApprovalStep, error)
	Update(ctx context.Context, s *model.ApprovalStep) error
	ListByRequest(ctx context.Context, requestID string) ([]model.ApprovalStep, error)
	// GetCurrentPendingStep returns the lowest-order pending step for a request.
	GetCurrentPendingStep(ctx context.Context, requestID string) (*model.ApprovalStep, error)
}

// ApprovalHistoryRepository defines persistence for approval history.
type ApprovalHistoryRepository interface {
	Create(ctx context.Context, h *model.ApprovalHistory) error
	ListByRequest(ctx context.Context, requestID string) ([]model.ApprovalHistory, error)
}
