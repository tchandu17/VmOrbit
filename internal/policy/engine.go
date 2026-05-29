// Package policy implements the VMOrbit policy evaluation engine.
// It intercepts infrastructure operations and applies governance rules
// defined by administrators.
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// Engine evaluates policies against operation requests.
type Engine struct {
	policyRepo     port.PolicyRepository
	assignmentRepo port.PolicyAssignmentRepository
	violationRepo  port.PolicyViolationRepository
	approvalSvc    port.ApprovalService
	log            logger.Logger
}

// NewEngine creates a new policy evaluation engine.
func NewEngine(
	policyRepo port.PolicyRepository,
	assignmentRepo port.PolicyAssignmentRepository,
	violationRepo port.PolicyViolationRepository,
	approvalSvc port.ApprovalService,
	log logger.Logger,
) *Engine {
	return &Engine{
		policyRepo:     policyRepo,
		assignmentRepo: assignmentRepo,
		violationRepo:  violationRepo,
		approvalSvc:    approvalSvc,
		log:            log,
	}
}

// Evaluate runs all applicable policies against the request and returns the
// enforcement decision. The caller must check result.Allowed before proceeding.
func (e *Engine) Evaluate(ctx context.Context, req port.PolicyEvaluationRequest) (*port.PolicyEvaluationResult, error) {
	// Load all enabled policies ordered by priority.
	policies, err := e.policyRepo.ListEnabled(ctx)
	if err != nil {
		// Policy engine failures must not block operations — log and allow.
		e.log.Error("policy engine: failed to load policies",
			logger.Error(err),
			logger.String("operation", req.Operation),
		)
		return &port.PolicyEvaluationResult{Allowed: true, Message: "policy engine unavailable"}, nil
	}

	if len(policies) == 0 {
		return &port.PolicyEvaluationResult{Allowed: true, Message: "no policies configured"}, nil
	}

	// Build the set of target IDs relevant to this request for assignment lookup.
	targetIDs := e.buildTargetIDs(req)

	// Load assignments for this context.
	assignments, err := e.assignmentRepo.ListForContext(ctx, targetIDs)
	if err != nil {
		e.log.Error("policy engine: failed to load assignments", logger.Error(err))
		return &port.PolicyEvaluationResult{Allowed: true, Message: "policy engine unavailable"}, nil
	}

	// Build a set of policy IDs that are assigned to this context.
	assignedPolicyIDs := make(map[string]bool)
	for _, a := range assignments {
		assignedPolicyIDs[a.PolicyID.String()] = true
	}

	// Evaluate each policy in priority order.
	for i := range policies {
		p := &policies[i]

		// Skip if this policy is not assigned to the current context.
		if !assignedPolicyIDs[p.ID.String()] {
			continue
		}

		// Check if this policy applies to the requested operation.
		if !e.operationMatches(p.Operations, req.Operation) {
			continue
		}

		// Evaluate all conditions (AND logic).
		if !e.evaluateConditions(p.Conditions, req) {
			continue
		}

		// Policy matched — apply the effect.
		return e.applyEffect(ctx, p, req)
	}

	// No policy matched — default allow.
	return &port.PolicyEvaluationResult{Allowed: true, Message: "no matching policy"}, nil
}

// ── Effect application ────────────────────────────────────────────────────────

func (e *Engine) applyEffect(ctx context.Context, p *model.Policy, req port.PolicyEvaluationRequest) (*port.PolicyEvaluationResult, error) {
	result := &port.PolicyEvaluationResult{
		Effect:        p.Effect,
		MatchedPolicy: p,
	}

	switch p.Effect {
	case model.PolicyEffectAllow:
		result.Allowed = true
		result.Message = fmt.Sprintf("allowed by policy %q", p.Name)

	case model.PolicyEffectDeny:
		result.Allowed = false
		result.Message = fmt.Sprintf("denied by policy %q", p.Name)
		// Record violation
		vid, _ := e.recordViolation(ctx, p, req, model.PolicyViolationStatusBlocked, "")
		result.ViolationID = vid

	case model.PolicyEffectRequireApproval:
		result.Allowed = false
		result.Message = fmt.Sprintf("approval required by policy %q", p.Name)
		// Create approval request
		approvalID, err := e.createApprovalRequest(ctx, p, req)
		if err != nil {
			e.log.Error("policy engine: failed to create approval request",
				logger.Error(err),
				logger.String("policy", p.Name),
			)
			result.Message = fmt.Sprintf("denied by policy %q (approval creation failed)", p.Name)
		} else {
			result.ApprovalRequestID = approvalID
		}
		// Record violation
		vid, _ := e.recordViolation(ctx, p, req, model.PolicyViolationStatusPending, approvalID)
		result.ViolationID = vid

	case model.PolicyEffectRequireSnapshot:
		result.Allowed = true // operation is allowed but snapshot must be taken first
		result.RequiresSnapshot = true
		result.Message = fmt.Sprintf("snapshot required before operation by policy %q", p.Name)

	case model.PolicyEffectRequireJustification:
		result.Allowed = false // caller must re-submit with justification
		result.RequiresJustification = true
		result.Message = fmt.Sprintf("justification required by policy %q", p.Name)
		// If justification was already provided in the request metadata, allow it.
		if justification, ok := req.Metadata["justification"].(string); ok && justification != "" {
			result.Allowed = true
			result.Message = fmt.Sprintf("allowed with justification by policy %q", p.Name)
			vid, _ := e.recordViolation(ctx, p, req, model.PolicyViolationStatusOverridden, "")
			result.ViolationID = vid
		}
	}

	e.log.Info("policy evaluated",
		logger.String("policy", p.Name),
		logger.String("effect", string(p.Effect)),
		logger.String("operation", req.Operation),
		logger.String("resource", req.ResourceID),
		logger.Bool("allowed", result.Allowed),
	)

	return result, nil
}

// ── Condition evaluation ──────────────────────────────────────────────────────

// evaluateConditions returns true if ALL conditions are satisfied.
func (e *Engine) evaluateConditions(conditions []model.PolicyCondition, req port.PolicyEvaluationRequest) bool {
	for _, c := range conditions {
		matched := e.evaluateCondition(c, req)
		if c.Negate {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}

func (e *Engine) evaluateCondition(c model.PolicyCondition, req port.PolicyEvaluationRequest) bool {
	switch c.Type {
	case model.PolicyConditionVMTag:
		return e.matchStringSlice(c.Operator, c.Value, req.VMTags)

	case model.PolicyConditionEnvironment:
		return e.matchString(c.Operator, c.Value, req.Environment)

	case model.PolicyConditionProvider:
		return e.matchString(c.Operator, c.Value, req.ProviderType)

	case model.PolicyConditionUserRole:
		return e.matchStringSlice(c.Operator, c.Value, req.Roles)

	case model.PolicyConditionOperation:
		return e.matchString(c.Operator, c.Value, req.Operation)

	case model.PolicyConditionVMName:
		return e.matchString(c.Operator, c.Value, req.ResourceName)

	case model.PolicyConditionHypervisor:
		return e.matchString(c.Operator, c.Value, req.HypervisorID)

	case model.PolicyConditionBulkSize:
		threshold, err := strconv.Atoi(c.Value)
		if err != nil {
			return false
		}
		switch c.Operator {
		case model.PolicyConditionOpGreaterThan:
			return req.BulkSize > threshold
		case model.PolicyConditionOpLessThan:
			return req.BulkSize < threshold
		case model.PolicyConditionOpEquals:
			return req.BulkSize == threshold
		}
		return false

	case model.PolicyConditionMaintenanceWindow:
		return e.isInMaintenanceWindow(c.Value)

	case model.PolicyConditionTimeSchedule:
		return e.isInTimeSchedule(c.Value)
	}

	return false
}

// matchString evaluates a string condition against a single value.
func (e *Engine) matchString(op model.PolicyConditionOperator, condValue, actual string) bool {
	switch op {
	case model.PolicyConditionOpEquals:
		return strings.EqualFold(actual, condValue)
	case model.PolicyConditionOpNotEquals:
		return !strings.EqualFold(actual, condValue)
	case model.PolicyConditionOpContains:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(condValue))
	case model.PolicyConditionOpIn:
		var values []string
		if err := json.Unmarshal([]byte(condValue), &values); err != nil {
			// Fall back to comma-separated
			values = strings.Split(condValue, ",")
		}
		for _, v := range values {
			if strings.EqualFold(strings.TrimSpace(v), actual) {
				return true
			}
		}
		return false
	case model.PolicyConditionOpNotIn:
		var values []string
		if err := json.Unmarshal([]byte(condValue), &values); err != nil {
			values = strings.Split(condValue, ",")
		}
		for _, v := range values {
			if strings.EqualFold(strings.TrimSpace(v), actual) {
				return false
			}
		}
		return true
	case model.PolicyConditionOpMatches:
		re, err := regexp.Compile(condValue)
		if err != nil {
			return false
		}
		return re.MatchString(actual)
	}
	return false
}

// matchStringSlice evaluates a condition against a slice of values.
func (e *Engine) matchStringSlice(op model.PolicyConditionOperator, condValue string, actuals []string) bool {
	switch op {
	case model.PolicyConditionOpContains, model.PolicyConditionOpEquals:
		for _, a := range actuals {
			if strings.EqualFold(a, condValue) {
				return true
			}
		}
		return false
	case model.PolicyConditionOpIn:
		var values []string
		if err := json.Unmarshal([]byte(condValue), &values); err != nil {
			values = strings.Split(condValue, ",")
		}
		for _, v := range values {
			for _, a := range actuals {
				if strings.EqualFold(strings.TrimSpace(v), a) {
					return true
				}
			}
		}
		return false
	case model.PolicyConditionOpNotIn:
		var values []string
		if err := json.Unmarshal([]byte(condValue), &values); err != nil {
			values = strings.Split(condValue, ",")
		}
		for _, v := range values {
			for _, a := range actuals {
				if strings.EqualFold(strings.TrimSpace(v), a) {
					return false
				}
			}
		}
		return true
	}
	return false
}

// isInMaintenanceWindow checks if the current time falls within a maintenance window.
// condValue format: "HH:MM-HH:MM" (daily window) or JSON with days/times.
func (e *Engine) isInMaintenanceWindow(condValue string) bool {
	now := time.Now().UTC()
	// Simple HH:MM-HH:MM format
	parts := strings.Split(condValue, "-")
	if len(parts) == 2 {
		start, err1 := time.Parse("15:04", strings.TrimSpace(parts[0]))
		end, err2 := time.Parse("15:04", strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			return false
		}
		nowTime, _ := time.Parse("15:04", now.Format("15:04"))
		return !nowTime.Before(start) && nowTime.Before(end)
	}
	return false
}

// isInTimeSchedule checks if the current time matches a schedule expression.
// condValue format: JSON {"days": ["monday","tuesday"], "start": "09:00", "end": "17:00"}
func (e *Engine) isInTimeSchedule(condValue string) bool {
	var schedule struct {
		Days  []string `json:"days"`
		Start string   `json:"start"`
		End   string   `json:"end"`
	}
	if err := json.Unmarshal([]byte(condValue), &schedule); err != nil {
		return false
	}

	now := time.Now().UTC()
	currentDay := strings.ToLower(now.Weekday().String())

	dayMatches := false
	for _, d := range schedule.Days {
		if strings.EqualFold(d, currentDay) {
			dayMatches = true
			break
		}
	}
	if !dayMatches {
		return false
	}

	if schedule.Start != "" && schedule.End != "" {
		start, err1 := time.Parse("15:04", schedule.Start)
		end, err2 := time.Parse("15:04", schedule.End)
		if err1 != nil || err2 != nil {
			return false
		}
		nowTime, _ := time.Parse("15:04", now.Format("15:04"))
		return !nowTime.Before(start) && nowTime.Before(end)
	}

	return true
}

// ── Operation matching ────────────────────────────────────────────────────────

// operationMatches returns true if the operation is in the policy's operations list.
// An empty list or a list containing "*" matches all operations.
func (e *Engine) operationMatches(operations model.StringArray, operation string) bool {
	if len(operations) == 0 {
		return true
	}
	for _, op := range operations {
		if op == "*" || strings.EqualFold(op, operation) {
			return true
		}
		// Support prefix matching: "vm.*" matches "vm.power_on", "vm.delete", etc.
		if strings.HasSuffix(op, ".*") {
			prefix := strings.TrimSuffix(op, ".*")
			if strings.HasPrefix(operation, prefix+".") || strings.EqualFold(operation, prefix) {
				return true
			}
		}
	}
	return false
}

// ── Target ID building ────────────────────────────────────────────────────────

// buildTargetIDs returns the set of target IDs relevant to this request.
// Used to look up applicable policy assignments.
func (e *Engine) buildTargetIDs(req port.PolicyEvaluationRequest) []string {
	var ids []string
	if req.ResourceID != "" {
		ids = append(ids, req.ResourceID)
	}
	if req.HypervisorID != "" {
		ids = append(ids, req.HypervisorID)
	}
	if req.Environment != "" {
		ids = append(ids, req.Environment)
	}
	for _, tag := range req.VMTags {
		ids = append(ids, tag)
	}
	for _, role := range req.Roles {
		ids = append(ids, role)
	}
	return ids
}

// ── Violation recording ───────────────────────────────────────────────────────

func (e *Engine) recordViolation(
	ctx context.Context,
	p *model.Policy,
	req port.PolicyEvaluationRequest,
	status model.PolicyViolationStatus,
	approvalID string,
) (string, error) {
	v := &model.PolicyViolation{
		PolicyID:     p.ID,
		PolicyName:   p.Name,
		Effect:       p.Effect,
		Status:       status,
		Operation:    req.Operation,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		Username:     req.Username,
		Metadata:     req.Metadata,
	}

	if req.UserID != "" {
		if uid, err := uuid.Parse(req.UserID); err == nil {
			v.UserID = &uid
		}
	}

	if approvalID != "" {
		if aid, err := uuid.Parse(approvalID); err == nil {
			v.ApprovalRequestID = &aid
		}
	}

	if err := e.violationRepo.Create(ctx, v); err != nil {
		e.log.Warn("policy engine: failed to record violation",
			logger.Error(err),
			logger.String("policy", p.Name),
		)
		return "", err
	}

	return v.ID.String(), nil
}

// ── Approval request creation ─────────────────────────────────────────────────

func (e *Engine) createApprovalRequest(ctx context.Context, p *model.Policy, req port.PolicyEvaluationRequest) (string, error) {
	// Build approval steps from policy config.
	steps := e.buildApprovalSteps(p)

	// Determine expiry from policy config.
	var expiresIn time.Duration
	if p.ApprovalConfig != nil {
		if ttlHours, ok := p.ApprovalConfig["expiry_hours"].(float64); ok && ttlHours > 0 {
			expiresIn = time.Duration(ttlHours) * time.Hour
		}
	}
	if expiresIn == 0 {
		expiresIn = 24 * time.Hour // default 24h expiry
	}

	createReq := port.CreateApprovalRequest{
		PolicyID:         p.ID.String(),
		PolicyName:       p.Name,
		Operation:        req.Operation,
		ResourceType:     req.ResourceType,
		ResourceID:       req.ResourceID,
		ResourceName:     req.ResourceName,
		RequesterID:      req.UserID,
		RequesterName:    req.Username,
		OperationPayload: req.Metadata,
		Steps:            steps,
		ExpiresIn:        expiresIn,
	}

	if justification, ok := req.Metadata["justification"].(string); ok {
		createReq.Justification = justification
	}

	approval, err := e.approvalSvc.Create(ctx, createReq)
	if err != nil {
		return "", fmt.Errorf("creating approval request: %w", err)
	}

	return approval.ID.String(), nil
}

func (e *Engine) buildApprovalSteps(p *model.Policy) []port.CreateApprovalStepRequest {
	if p.ApprovalConfig == nil {
		return nil
	}

	// Parse approval_steps from policy config.
	// Format: [{"order": 1, "approver_role": "admin"}, {"order": 2, "approver_id": "uuid"}]
	stepsRaw, ok := p.ApprovalConfig["approval_steps"]
	if !ok {
		// Fall back to single-step with approver_role
		if role, ok := p.ApprovalConfig["approver_role"].(string); ok && role != "" {
			return []port.CreateApprovalStepRequest{
				{StepOrder: 1, ApproverRole: role},
			}
		}
		return nil
	}

	stepsJSON, err := json.Marshal(stepsRaw)
	if err != nil {
		return nil
	}

	var rawSteps []struct {
		Order        int    `json:"order"`
		ApproverID   string `json:"approver_id"`
		ApproverRole string `json:"approver_role"`
		ApproverName string `json:"approver_name"`
	}
	if err := json.Unmarshal(stepsJSON, &rawSteps); err != nil {
		return nil
	}

	steps := make([]port.CreateApprovalStepRequest, 0, len(rawSteps))
	for _, s := range rawSteps {
		steps = append(steps, port.CreateApprovalStepRequest{
			StepOrder:    s.Order,
			ApproverID:   s.ApproverID,
			ApproverRole: s.ApproverRole,
			ApproverName: s.ApproverName,
		})
	}
	return steps
}
