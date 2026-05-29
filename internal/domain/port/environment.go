package port

import (
	"context"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Repository interfaces
// ─────────────────────────────────────────────────────────────────────────────

// EnvironmentRepository defines persistence operations for environments.
type EnvironmentRepository interface {
	Create(ctx context.Context, env *model.Environment) error
	GetByID(ctx context.Context, id string) (*model.Environment, error)
	Update(ctx context.Context, env *model.Environment) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter EnvironmentFilter, page Page) (*PageResult[model.Environment], error)
	// UpdateStatus updates the aggregate status of an environment.
	UpdateStatus(ctx context.Context, id string, status model.EnvironmentStatus) error
}

// EnvironmentVMRepository defines persistence for environment ↔ VM associations.
type EnvironmentVMRepository interface {
	// AddVM adds a VM to an environment (idempotent — updates ordering if already present).
	AddVM(ctx context.Context, ev *model.EnvironmentVM) error
	// RemoveVM removes a VM from an environment.
	RemoveVM(ctx context.Context, environmentID, vmID string) error
	// ListByEnvironment returns all EnvironmentVM records for an environment, ordered by StartOrder.
	ListByEnvironment(ctx context.Context, environmentID string) ([]model.EnvironmentVM, error)
	// UpdateOrdering updates the start/stop order and role for a VM in an environment.
	UpdateOrdering(ctx context.Context, environmentID, vmID string, startOrder, stopOrder int, role string) error
	// GetByEnvironmentAndVM returns a single EnvironmentVM record.
	GetByEnvironmentAndVM(ctx context.Context, environmentID, vmID string) (*model.EnvironmentVM, error)
}

// VMDependencyRepository defines persistence for VM dependency edges.
type VMDependencyRepository interface {
	Create(ctx context.Context, dep *model.VMDependency) error
	GetByID(ctx context.Context, id string) (*model.VMDependency, error)
	Delete(ctx context.Context, id string) error
	// ListByEnvironment returns all dependency edges for an environment.
	ListByEnvironment(ctx context.Context, environmentID string) ([]model.VMDependency, error)
	// ListDependenciesOf returns edges where TargetVMID = vmID (things this VM depends on).
	ListDependenciesOf(ctx context.Context, environmentID, vmID string) ([]model.VMDependency, error)
	// ListDependentsOf returns edges where SourceVMID = vmID (things that depend on this VM).
	ListDependentsOf(ctx context.Context, environmentID, vmID string) ([]model.VMDependency, error)
}

// OrchestrationRunRepository defines persistence for orchestration runs.
type OrchestrationRunRepository interface {
	Create(ctx context.Context, run *model.OrchestrationRun) error
	GetByID(ctx context.Context, id string) (*model.OrchestrationRun, error)
	Update(ctx context.Context, run *model.OrchestrationRun) error
	List(ctx context.Context, environmentID string, page Page) (*PageResult[model.OrchestrationRun], error)
	// UpdateProgress updates progress, completed/failed/skipped VM counts.
	UpdateProgress(ctx context.Context, id string, progress, completed, failed, skipped int) error
	// UpdateStatus updates the run status and optional error message.
	UpdateStatus(ctx context.Context, id string, status model.OrchestrationRunStatus, errMsg string) error
}

// OrchestrationStepRepository defines persistence for orchestration steps.
type OrchestrationStepRepository interface {
	CreateBatch(ctx context.Context, steps []model.OrchestrationStep) error
	GetByID(ctx context.Context, id string) (*model.OrchestrationStep, error)
	ListByRun(ctx context.Context, runID string) ([]model.OrchestrationStep, error)
	UpdateStatus(ctx context.Context, id string, status model.OrchestrationStepStatus, taskID *string, errMsg string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Service interface
// ─────────────────────────────────────────────────────────────────────────────

// EnvironmentService manages environment lifecycle and orchestration.
type EnvironmentService interface {
	// ── CRUD ──────────────────────────────────────────────────────────────────
	Create(ctx context.Context, req CreateEnvironmentRequest) (*model.Environment, error)
	GetByID(ctx context.Context, id string) (*model.Environment, error)
	Update(ctx context.Context, id string, req UpdateEnvironmentRequest) (*model.Environment, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter EnvironmentFilter, page Page) (*PageResult[model.Environment], error)

	// ── VM membership ─────────────────────────────────────────────────────────
	AddVM(ctx context.Context, environmentID, vmID string, req AddVMToEnvironmentRequest) error
	RemoveVM(ctx context.Context, environmentID, vmID string) error
	UpdateVMOrdering(ctx context.Context, environmentID, vmID string, req UpdateVMOrderingRequest) error
	ListVMs(ctx context.Context, environmentID string) ([]model.EnvironmentVM, error)

	// ── Dependencies ──────────────────────────────────────────────────────────
	AddDependency(ctx context.Context, environmentID string, req AddDependencyRequest) (*model.VMDependency, error)
	RemoveDependency(ctx context.Context, dependencyID string) error
	ListDependencies(ctx context.Context, environmentID string) ([]model.VMDependency, error)

	// ── Orchestration ─────────────────────────────────────────────────────────
	// StartEnvironment powers on all VMs in dependency-aware order. Returns run ID.
	StartEnvironment(ctx context.Context, id string) (string, error)
	// StopEnvironment powers off all VMs in reverse dependency order. Returns run ID.
	StopEnvironment(ctx context.Context, id string) (string, error)
	// RestartEnvironment stops then starts the environment. Returns run ID.
	RestartEnvironment(ctx context.Context, id string) (string, error)
	// SnapshotEnvironment creates a snapshot of all VMs. Returns run ID.
	SnapshotEnvironment(ctx context.Context, id string, req EnvironmentSnapshotRequest) (string, error)
	// CloneEnvironment clones all VMs into a new environment. Returns run ID.
	CloneEnvironment(ctx context.Context, id string, req EnvironmentCloneRequest) (string, error)

	// ── Run tracking ──────────────────────────────────────────────────────────
	GetRun(ctx context.Context, runID string) (*model.OrchestrationRun, error)
	ListRuns(ctx context.Context, environmentID string, page Page) (*PageResult[model.OrchestrationRun], error)
	GetRunSteps(ctx context.Context, runID string) ([]model.OrchestrationStep, error)

	// ── Health ────────────────────────────────────────────────────────────────
	// RefreshStatus recomputes the aggregate status from VM states and updates the DB.
	RefreshStatus(ctx context.Context, id string) (*model.Environment, error)

	// ── Internal helpers used by the task engine ──────────────────────────────
	// UpdateRunStatus updates the status of an orchestration run.
	UpdateRunStatus(ctx context.Context, runID string, status model.OrchestrationRunStatus, errMsg string) error
	// UpdateRunProgress updates progress counters on an orchestration run.
	UpdateRunProgress(ctx context.Context, runID string, progress, completed, failed, skipped int) error
	// UpdateStepStatus updates the status of an orchestration step.
	UpdateStepStatus(ctx context.Context, stepID string, status model.OrchestrationStepStatus, taskID *string, errMsg string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Request / filter types
// ─────────────────────────────────────────────────────────────────────────────

// EnvironmentFilter narrows environment list queries.
type EnvironmentFilter struct {
	Type    string
	Status  string
	OwnerID string
	Search  string
}

// CreateEnvironmentRequest carries new-environment data.
type CreateEnvironmentRequest struct {
	Name        string
	Description string
	Type        model.EnvironmentType
	OwnerID     string
	Tags        []string
	Color       string
	Metadata    model.JSONMap
}

// UpdateEnvironmentRequest carries environment update data.
type UpdateEnvironmentRequest struct {
	Name        *string
	Description *string
	Type        *model.EnvironmentType
	OwnerID     *string
	Tags        []string
	Color       *string
	Metadata    model.JSONMap
}

// AddVMToEnvironmentRequest carries VM membership data.
type AddVMToEnvironmentRequest struct {
	StartOrder int
	StopOrder  int
	Role       string
	Notes      string
}

// UpdateVMOrderingRequest carries ordering update data.
type UpdateVMOrderingRequest struct {
	StartOrder int
	StopOrder  int
	Role       string
}

// AddDependencyRequest carries dependency edge data.
type AddDependencyRequest struct {
	SourceVMID   string
	TargetVMID   string
	Type         model.DependencyType
	DelaySeconds int
	Notes        string
}

// EnvironmentSnapshotRequest carries snapshot parameters for an environment operation.
type EnvironmentSnapshotRequest struct {
	SnapshotName string
	Description  string
	Memory       bool
}

// EnvironmentCloneRequest carries clone parameters for an environment operation.
type EnvironmentCloneRequest struct {
	NewEnvironmentName string
	NameSuffix         string // appended to each cloned VM name
}
