package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// environmentService implements port.EnvironmentService.
type environmentService struct {
	envRepo    port.EnvironmentRepository
	envVMRepo  port.EnvironmentVMRepository
	depRepo    port.VMDependencyRepository
	runRepo    port.OrchestrationRunRepository
	stepRepo   port.OrchestrationStepRepository
	vmRepo     port.VMRepository
	taskRepo   port.TaskRepository
	vmSvc      port.VMService
	audit      port.AuditService
	log        logger.Logger
}

// NewEnvironmentService creates a new environment service.
func NewEnvironmentService(
	envRepo port.EnvironmentRepository,
	envVMRepo port.EnvironmentVMRepository,
	depRepo port.VMDependencyRepository,
	runRepo port.OrchestrationRunRepository,
	stepRepo port.OrchestrationStepRepository,
	vmRepo port.VMRepository,
	taskRepo port.TaskRepository,
	vmSvc port.VMService,
	audit port.AuditService,
	log logger.Logger,
) port.EnvironmentService {
	return &environmentService{
		envRepo:   envRepo,
		envVMRepo: envVMRepo,
		depRepo:   depRepo,
		runRepo:   runRepo,
		stepRepo:  stepRepo,
		vmRepo:    vmRepo,
		taskRepo:  taskRepo,
		vmSvc:     vmSvc,
		audit:     audit,
		log:       log,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CRUD
// ─────────────────────────────────────────────────────────────────────────────

func (s *environmentService) Create(ctx context.Context, req port.CreateEnvironmentRequest) (*model.Environment, error) {
	envType := req.Type
	if envType == "" {
		envType = model.EnvironmentTypeCustom
	}

	var ownerID *uuid.UUID
	if req.OwnerID != "" {
		id, err := uuid.Parse(req.OwnerID)
		if err == nil {
			ownerID = &id
		}
	}

	env := &model.Environment{
		Name:        req.Name,
		Description: req.Description,
		Type:        envType,
		Status:      model.EnvironmentStatusUnknown,
		OwnerID:     ownerID,
		Tags:        req.Tags,
		Color:       req.Color,
		Metadata:    req.Metadata,
	}

	if err := s.envRepo.Create(ctx, env); err != nil {
		return nil, fmt.Errorf("creating environment: %w", err)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionCreate,
		Resource:   "environment",
		ResourceID: env.ID.String(),
		Success:    true,
	})

	return env, nil
}

func (s *environmentService) GetByID(ctx context.Context, id string) (*model.Environment, error) {
	return s.envRepo.GetByID(ctx, id)
}

func (s *environmentService) Update(ctx context.Context, id string, req port.UpdateEnvironmentRequest) (*model.Environment, error) {
	env, err := s.envRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		env.Name = *req.Name
	}
	if req.Description != nil {
		env.Description = *req.Description
	}
	if req.Type != nil {
		env.Type = *req.Type
	}
	if req.Color != nil {
		env.Color = *req.Color
	}
	if req.Tags != nil {
		env.Tags = req.Tags
	}
	if req.OwnerID != nil {
		if *req.OwnerID == "" {
			env.OwnerID = nil
		} else {
			id, err := uuid.Parse(*req.OwnerID)
			if err == nil {
				env.OwnerID = &id
			}
		}
	}
	if req.Metadata != nil {
		if env.Metadata == nil {
			env.Metadata = model.JSONMap{}
		}
		for k, v := range req.Metadata {
			env.Metadata[k] = v
		}
	}

	if err := s.envRepo.Update(ctx, env); err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionUpdate,
		Resource:   "environment",
		ResourceID: id,
		Success:    true,
	})

	return env, nil
}

func (s *environmentService) Delete(ctx context.Context, id string) error {
	if err := s.envRepo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionDelete,
		Resource:   "environment",
		ResourceID: id,
		Success:    true,
	})
	return nil
}

func (s *environmentService) List(ctx context.Context, filter port.EnvironmentFilter, page port.Page) (*port.PageResult[model.Environment], error) {
	return s.envRepo.List(ctx, filter, page)
}

// ─────────────────────────────────────────────────────────────────────────────
// VM membership
// ─────────────────────────────────────────────────────────────────────────────

func (s *environmentService) AddVM(ctx context.Context, environmentID, vmID string, req port.AddVMToEnvironmentRequest) error {
	// Verify both exist
	if _, err := s.envRepo.GetByID(ctx, environmentID); err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}
	if _, err := s.vmRepo.GetByID(ctx, vmID); err != nil {
		return fmt.Errorf("vm not found: %w", err)
	}

	envUUID, err := uuid.Parse(environmentID)
	if err != nil {
		return fmt.Errorf("invalid environment id: %w", err)
	}
	vmUUID, err := uuid.Parse(vmID)
	if err != nil {
		return fmt.Errorf("invalid vm id: %w", err)
	}

	ev := &model.EnvironmentVM{
		EnvironmentID: envUUID,
		VMID:          vmUUID,
		StartOrder:    req.StartOrder,
		StopOrder:     req.StopOrder,
		Role:          req.Role,
		Notes:         req.Notes,
	}

	if err := s.envVMRepo.AddVM(ctx, ev); err != nil {
		return fmt.Errorf("adding vm to environment: %w", err)
	}

	// Refresh aggregate status
	go func() {
		_, _ = s.RefreshStatus(context.Background(), environmentID)
	}()

	return nil
}

func (s *environmentService) RemoveVM(ctx context.Context, environmentID, vmID string) error {
	if err := s.envVMRepo.RemoveVM(ctx, environmentID, vmID); err != nil {
		return fmt.Errorf("removing vm from environment: %w", err)
	}
	// Also remove any dependencies involving this VM in this environment
	deps, err := s.depRepo.ListByEnvironment(ctx, environmentID)
	if err == nil {
		for _, dep := range deps {
			if dep.SourceVMID.String() == vmID || dep.TargetVMID.String() == vmID {
				_ = s.depRepo.Delete(ctx, dep.ID.String())
			}
		}
	}
	go func() {
		_, _ = s.RefreshStatus(context.Background(), environmentID)
	}()
	return nil
}

func (s *environmentService) UpdateVMOrdering(ctx context.Context, environmentID, vmID string, req port.UpdateVMOrderingRequest) error {
	return s.envVMRepo.UpdateOrdering(ctx, environmentID, vmID, req.StartOrder, req.StopOrder, req.Role)
}

func (s *environmentService) ListVMs(ctx context.Context, environmentID string) ([]model.EnvironmentVM, error) {
	return s.envVMRepo.ListByEnvironment(ctx, environmentID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Dependencies
// ─────────────────────────────────────────────────────────────────────────────

func (s *environmentService) AddDependency(ctx context.Context, environmentID string, req port.AddDependencyRequest) (*model.VMDependency, error) {
	envUUID, err := uuid.Parse(environmentID)
	if err != nil {
		return nil, fmt.Errorf("invalid environment id: %w", err)
	}
	srcUUID, err := uuid.Parse(req.SourceVMID)
	if err != nil {
		return nil, fmt.Errorf("invalid source_vm_id: %w", err)
	}
	tgtUUID, err := uuid.Parse(req.TargetVMID)
	if err != nil {
		return nil, fmt.Errorf("invalid target_vm_id: %w", err)
	}
	if srcUUID == tgtUUID {
		return nil, fmt.Errorf("source and target vm cannot be the same")
	}

	dep := &model.VMDependency{
		EnvironmentID: envUUID,
		SourceVMID:    srcUUID,
		TargetVMID:    tgtUUID,
		Type:          req.Type,
		DelaySeconds:  req.DelaySeconds,
		Notes:         req.Notes,
	}

	if err := s.depRepo.Create(ctx, dep); err != nil {
		return nil, fmt.Errorf("creating dependency: %w", err)
	}

	return dep, nil
}

func (s *environmentService) RemoveDependency(ctx context.Context, dependencyID string) error {
	return s.depRepo.Delete(ctx, dependencyID)
}

func (s *environmentService) ListDependencies(ctx context.Context, environmentID string) ([]model.VMDependency, error) {
	return s.depRepo.ListByEnvironment(ctx, environmentID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Orchestration — start / stop / restart / snapshot / clone
// ─────────────────────────────────────────────────────────────────────────────

func (s *environmentService) StartEnvironment(ctx context.Context, id string) (string, error) {
	return s.createOrchestrationRun(ctx, id, model.OrchestrationOpStart, nil)
}

func (s *environmentService) StopEnvironment(ctx context.Context, id string) (string, error) {
	return s.createOrchestrationRun(ctx, id, model.OrchestrationOpStop, nil)
}

func (s *environmentService) RestartEnvironment(ctx context.Context, id string) (string, error) {
	return s.createOrchestrationRun(ctx, id, model.OrchestrationOpRestart, nil)
}

func (s *environmentService) SnapshotEnvironment(ctx context.Context, id string, req port.EnvironmentSnapshotRequest) (string, error) {
	payload := model.JSONMap{
		"snapshot_name": req.SnapshotName,
		"description":   req.Description,
		"memory":        req.Memory,
	}
	return s.createOrchestrationRun(ctx, id, model.OrchestrationOpSnapshot, payload)
}

func (s *environmentService) CloneEnvironment(ctx context.Context, id string, req port.EnvironmentCloneRequest) (string, error) {
	payload := model.JSONMap{
		"new_environment_name": req.NewEnvironmentName,
		"name_suffix":          req.NameSuffix,
	}
	return s.createOrchestrationRun(ctx, id, model.OrchestrationOpClone, payload)
}

// createOrchestrationRun creates an OrchestrationRun record and its steps,
// then creates a parent task in the DB. The caller must enqueue the task.
func (s *environmentService) createOrchestrationRun(
	ctx context.Context,
	environmentID string,
	op model.OrchestrationOperation,
	payload model.JSONMap,
) (string, error) {
	env, err := s.envRepo.GetByID(ctx, environmentID)
	if err != nil {
		return "", fmt.Errorf("environment not found: %w", err)
	}

	// Load VMs in this environment
	envVMs, err := s.envVMRepo.ListByEnvironment(ctx, environmentID)
	if err != nil {
		return "", fmt.Errorf("loading environment vms: %w", err)
	}
	if len(envVMs) == 0 {
		return "", fmt.Errorf("environment has no VMs")
	}

	// Load dependencies
	deps, err := s.depRepo.ListByEnvironment(ctx, environmentID)
	if err != nil {
		return "", fmt.Errorf("loading dependencies: %w", err)
	}

	// Compute execution order based on operation type
	orderedVMs := computeExecutionOrder(envVMs, deps, op)

	// Create the orchestration run record
	envUUID, _ := uuid.Parse(environmentID)
	now := time.Now().UTC()
	run := &model.OrchestrationRun{
		EnvironmentID: envUUID,
		Operation:     op,
		Status:        model.OrchestrationRunStatusPending,
		TotalVMs:      len(orderedVMs),
		Payload:       payload,
		CreatedBy:     callerUUID(ctx),
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return "", fmt.Errorf("creating orchestration run: %w", err)
	}

	// Create steps
	steps := make([]model.OrchestrationStep, 0, len(orderedVMs))
	for i, ev := range orderedVMs {
		steps = append(steps, model.OrchestrationStep{
			RunID:          run.ID,
			VMID:           ev.VMID,
			ExecutionOrder: i,
			Status:         model.OrchestrationStepStatusPending,
		})
	}
	if err := s.stepRepo.CreateBatch(ctx, steps); err != nil {
		return "", fmt.Errorf("creating orchestration steps: %w", err)
	}

	// Map task type
	taskType := orchestrationTaskType(op)

	// Build payload for the task engine
	taskPayload := model.JSONMap{
		"environment_id": environmentID,
		"run_id":         run.ID.String(),
		"operation":      string(op),
	}
	if payload != nil {
		for k, v := range payload {
			taskPayload[k] = v
		}
	}

	// Create the parent task
	taskID := uuid.New()
	envUUID2, _ := uuid.Parse(environmentID)
	t := &model.Task{
		Type:         taskType,
		Status:       model.TaskStatusPending,
		Priority:     5,
		MaxRetries:   1,
		HypervisorID: nil,
		ScheduledAt:  &now,
		Payload:      taskPayload,
		CreatedBy:    callerUUID(ctx),
	}
	t.ID = taskID
	// Store environment_id in VMID field for scoping (reuse existing index)
	_ = envUUID2 // suppress unused warning

	if err := s.taskRepo.Create(ctx, t); err != nil {
		return "", fmt.Errorf("creating orchestration task: %w", err)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "environment",
		ResourceID:  environmentID,
		Description: fmt.Sprintf("orchestration %s started for environment %s", op, env.Name),
		Success:     true,
	})

	return taskID.String(), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Run tracking
// ─────────────────────────────────────────────────────────────────────────────

func (s *environmentService) GetRun(ctx context.Context, runID string) (*model.OrchestrationRun, error) {
	return s.runRepo.GetByID(ctx, runID)
}

func (s *environmentService) ListRuns(ctx context.Context, environmentID string, page port.Page) (*port.PageResult[model.OrchestrationRun], error) {
	return s.runRepo.List(ctx, environmentID, page)
}

func (s *environmentService) GetRunSteps(ctx context.Context, runID string) ([]model.OrchestrationStep, error) {
	return s.stepRepo.ListByRun(ctx, runID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Health aggregation
// ─────────────────────────────────────────────────────────────────────────────

func (s *environmentService) RefreshStatus(ctx context.Context, id string) (*model.Environment, error) {
	envVMs, err := s.envVMRepo.ListByEnvironment(ctx, id)
	if err != nil {
		return nil, err
	}

	if len(envVMs) == 0 {
		_ = s.envRepo.UpdateStatus(ctx, id, model.EnvironmentStatusUnknown)
		return s.envRepo.GetByID(ctx, id)
	}

	running, stopped, error_, total := 0, 0, 0, len(envVMs)
	for _, ev := range envVMs {
		switch ev.VM.Status {
		case model.VMStatusRunning:
			running++
		case model.VMStatusStopped:
			stopped++
		case model.VMStatusError:
			error_++
		}
	}

	var status model.EnvironmentStatus
	switch {
	case error_ > 0:
		status = model.EnvironmentStatusUnhealthy
	case running == total:
		status = model.EnvironmentStatusHealthy
	case stopped == total:
		status = model.EnvironmentStatusUnknown
	case running > 0:
		status = model.EnvironmentStatusDegraded
	default:
		status = model.EnvironmentStatusUnknown
	}

	_ = s.envRepo.UpdateStatus(ctx, id, status)
	return s.envRepo.GetByID(ctx, id)
}

// ─────────────────────────────────────────────────────────────────────────────
// Dependency-aware execution order computation
// ─────────────────────────────────────────────────────────────────────────────

// computeExecutionOrder returns envVMs sorted by execution order for the given
// operation. For start: lower StartOrder first, respecting dependencies.
// For stop: higher StopOrder first (reverse). For restart/snapshot/clone: start order.
func computeExecutionOrder(envVMs []model.EnvironmentVM, deps []model.VMDependency, op model.OrchestrationOperation) []model.EnvironmentVM {
	if op == model.OrchestrationOpStop {
		// Reverse stop order: higher StopOrder stops first
		sorted := make([]model.EnvironmentVM, len(envVMs))
		copy(sorted, envVMs)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].StopOrder > sorted[j].StopOrder
		})
		return sorted
	}

	// For start/restart/snapshot/clone: topological sort respecting start_before dependencies
	// Build adjacency: if A start_before B, then A must come before B (A → B edge)
	type edge struct{ from, to string }
	edges := make([]edge, 0)
	for _, dep := range deps {
		if dep.Type == model.DependencyTypeStartBefore || dep.Type == model.DependencyTypeRequires {
			// TargetVM must start before SourceVM
			edges = append(edges, edge{from: dep.TargetVMID.String(), to: dep.SourceVMID.String()})
		}
	}

	// Kahn's algorithm for topological sort
	vmIDs := make([]string, 0, len(envVMs))
	vmMap := make(map[string]model.EnvironmentVM, len(envVMs))
	for _, ev := range envVMs {
		vmIDs = append(vmIDs, ev.VMID.String())
		vmMap[ev.VMID.String()] = ev
	}

	inDegree := make(map[string]int, len(vmIDs))
	adj := make(map[string][]string, len(vmIDs))
	for _, id := range vmIDs {
		inDegree[id] = 0
	}
	for _, e := range edges {
		// Only consider edges between VMs in this environment
		if _, ok := vmMap[e.from]; !ok {
			continue
		}
		if _, ok := vmMap[e.to]; !ok {
			continue
		}
		adj[e.from] = append(adj[e.from], e.to)
		inDegree[e.to]++
	}

	// Start with nodes that have no incoming edges, sorted by StartOrder
	queue := make([]string, 0)
	for _, id := range vmIDs {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		return vmMap[queue[i]].StartOrder < vmMap[queue[j]].StartOrder
	})

	result := make([]model.EnvironmentVM, 0, len(envVMs))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, vmMap[cur])

		neighbors := adj[cur]
		sort.Slice(neighbors, func(i, j int) bool {
			return vmMap[neighbors[i]].StartOrder < vmMap[neighbors[j]].StartOrder
		})
		for _, next := range neighbors {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Append any remaining VMs not in the topological sort (cycle detection fallback)
	inResult := make(map[string]bool, len(result))
	for _, ev := range result {
		inResult[ev.VMID.String()] = true
	}
	remaining := make([]model.EnvironmentVM, 0)
	for _, ev := range envVMs {
		if !inResult[ev.VMID.String()] {
			remaining = append(remaining, ev)
		}
	}
	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].StartOrder < remaining[j].StartOrder
	})
	result = append(result, remaining...)

	return result
}

// orchestrationTaskType maps an operation to a task type.
func orchestrationTaskType(op model.OrchestrationOperation) model.TaskType {
	switch op {
	case model.OrchestrationOpStart:
		return model.TaskTypeEnvStart
	case model.OrchestrationOpStop:
		return model.TaskTypeEnvStop
	case model.OrchestrationOpRestart:
		return model.TaskTypeEnvRestart
	case model.OrchestrationOpSnapshot:
		return model.TaskTypeEnvSnapshot
	case model.OrchestrationOpClone:
		return model.TaskTypeEnvClone
	default:
		return model.TaskTypeEnvStart
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers used by the task engine
// ─────────────────────────────────────────────────────────────────────────────

func (s *environmentService) UpdateRunStatus(ctx context.Context, runID string, status model.OrchestrationRunStatus, errMsg string) error {
	updates := map[string]interface{}{"status": status}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	now := time.Now().UTC()
	switch status {
	case model.OrchestrationRunStatusRunning:
		updates["started_at"] = now
	case model.OrchestrationRunStatusCompleted,
		model.OrchestrationRunStatusFailed,
		model.OrchestrationRunStatusCancelled,
		model.OrchestrationRunStatusRolledBack:
		updates["completed_at"] = now
	}
	return s.runRepo.UpdateStatus(ctx, runID, status, errMsg)
}

func (s *environmentService) UpdateRunProgress(ctx context.Context, runID string, progress, completed, failed, skipped int) error {
	return s.runRepo.UpdateProgress(ctx, runID, progress, completed, failed, skipped)
}

func (s *environmentService) UpdateStepStatus(ctx context.Context, stepID string, status model.OrchestrationStepStatus, taskID *string, errMsg string) error {
	return s.stepRepo.UpdateStatus(ctx, stepID, status, taskID, errMsg)
}
