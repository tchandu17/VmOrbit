package model

import (
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// EnvironmentType classifies the purpose of an environment.
type EnvironmentType string

const (
	EnvironmentTypeProduction       EnvironmentType = "production"
	EnvironmentTypeStaging          EnvironmentType = "staging"
	EnvironmentTypeDevelopment      EnvironmentType = "development"
	EnvironmentTypeQA               EnvironmentType = "qa"
	EnvironmentTypeDisasterRecovery EnvironmentType = "disaster_recovery"
	EnvironmentTypeCustom           EnvironmentType = "custom"
)

// EnvironmentStatus reflects the aggregate health of an environment.
type EnvironmentStatus string

const (
	EnvironmentStatusHealthy   EnvironmentStatus = "healthy"
	EnvironmentStatusDegraded  EnvironmentStatus = "degraded"
	EnvironmentStatusUnhealthy EnvironmentStatus = "unhealthy"
	EnvironmentStatusUnknown   EnvironmentStatus = "unknown"
	EnvironmentStatusStarting  EnvironmentStatus = "starting"
	EnvironmentStatusStopping  EnvironmentStatus = "stopping"
)

// OrchestrationRunStatus tracks the lifecycle of an orchestration execution.
type OrchestrationRunStatus string

const (
	OrchestrationRunStatusPending     OrchestrationRunStatus = "pending"
	OrchestrationRunStatusRunning     OrchestrationRunStatus = "running"
	OrchestrationRunStatusCompleted   OrchestrationRunStatus = "completed"
	OrchestrationRunStatusFailed      OrchestrationRunStatus = "failed"
	OrchestrationRunStatusCancelled   OrchestrationRunStatus = "cancelled"
	OrchestrationRunStatusRollingBack OrchestrationRunStatus = "rolling_back"
	OrchestrationRunStatusRolledBack  OrchestrationRunStatus = "rolled_back"
)

// OrchestrationOperation is the type of orchestration being performed.
type OrchestrationOperation string

const (
	OrchestrationOpStart    OrchestrationOperation = "start"
	OrchestrationOpStop     OrchestrationOperation = "stop"
	OrchestrationOpRestart  OrchestrationOperation = "restart"
	OrchestrationOpSnapshot OrchestrationOperation = "snapshot"
	OrchestrationOpClone    OrchestrationOperation = "clone"
)

// DependencyType classifies the relationship between two VMs.
type DependencyType string

const (
	// DependencyTypeStartBefore: TargetVM must be running before SourceVM starts.
	DependencyTypeStartBefore DependencyType = "start_before"
	// DependencyTypeStopAfter: TargetVM must stop after SourceVM stops.
	DependencyTypeStopAfter DependencyType = "stop_after"
	// DependencyTypeRequires: SourceVM requires TargetVM to be running.
	DependencyTypeRequires DependencyType = "requires"
)

// ─────────────────────────────────────────────────────────────────────────────
// Environment
// ─────────────────────────────────────────────────────────────────────────────

// Environment is a logical grouping of VMs that form an application boundary.
// Examples: Production, Staging, DR, QA.
type Environment struct {
	Base
	Name        string            `gorm:"not null;size:128;uniqueIndex"            json:"name"`
	Description string            `gorm:"size:512"                                 json:"description"`
	Type        EnvironmentType   `gorm:"not null;default:'custom';size:32;index"  json:"type"`
	Status      EnvironmentStatus `gorm:"not null;default:'unknown';size:32;index" json:"status"`
	OwnerID     *uuid.UUID        `gorm:"type:uuid;index"                          json:"owner_id,omitempty"`
	Tags        StringArray       `gorm:"type:text[]"                              json:"tags"`
	Color       string            `gorm:"size:16"                                  json:"color"` // hex color for UI
	Metadata    JSONMap           `gorm:"type:jsonb"                               json:"metadata"`

	// Relations
	Owner   *User           `gorm:"foreignKey:OwnerID"       json:"owner,omitempty"`
	VMs     []EnvironmentVM `gorm:"foreignKey:EnvironmentID" json:"vms,omitempty"`
	EnvTags []EnvironmentTag `gorm:"foreignKey:EnvironmentID" json:"env_tags,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// EnvironmentVM — join table with ordering metadata
// ─────────────────────────────────────────────────────────────────────────────

// EnvironmentVM associates a VM with an environment and carries stack ordering.
// StartOrder and StopOrder define the sequence for orchestration operations.
// Lower numbers start first; higher numbers stop first (for shutdown).
type EnvironmentVM struct {
	Base
	EnvironmentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uidx_env_vm" json:"environment_id"`
	VMID          uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uidx_env_vm" json:"vm_id"`
	StartOrder    int       `gorm:"not null;default:0"                               json:"start_order"`
	StopOrder     int       `gorm:"not null;default:0"                               json:"stop_order"`
	Role          string    `gorm:"size:128"                                         json:"role"` // e.g. "database", "app", "lb"
	Notes         string    `gorm:"size:512"                                         json:"notes"`

	// Relations
	Environment Environment `gorm:"foreignKey:EnvironmentID" json:"environment,omitempty"`
	VM          VM          `gorm:"foreignKey:VMID"          json:"vm,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// EnvironmentTag — environment-level tags
// ─────────────────────────────────────────────────────────────────────────────

// EnvironmentTag attaches a Tag to an Environment.
type EnvironmentTag struct {
	EnvironmentID uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"environment_id"`
	TagID         uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"tag_id"`

	Environment Environment `gorm:"foreignKey:EnvironmentID" json:"-"`
	Tag         Tag         `gorm:"foreignKey:TagID"         json:"tag,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// VMDependency — directed dependency graph between VMs within an environment
// ─────────────────────────────────────────────────────────────────────────────

// VMDependency records that SourceVMID depends on TargetVMID.
// For DependencyTypeStartBefore: TargetVM must be running before SourceVM starts.
// For DependencyTypeStopAfter:   TargetVM must stop after SourceVM stops.
// For DependencyTypeRequires:    SourceVM requires TargetVM to be running.
type VMDependency struct {
	Base
	EnvironmentID uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:uidx_dep" json:"environment_id"`
	SourceVMID    uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:uidx_dep" json:"source_vm_id"`
	TargetVMID    uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:uidx_dep" json:"target_vm_id"`
	Type          DependencyType `gorm:"not null;size:32;uniqueIndex:uidx_dep"         json:"type"`
	DelaySeconds  int            `gorm:"not null;default:0"                            json:"delay_seconds"` // wait N seconds after target is up
	Notes         string         `gorm:"size:512"                                      json:"notes"`

	// Relations
	Environment Environment `gorm:"foreignKey:EnvironmentID" json:"environment,omitempty"`
	SourceVM    VM          `gorm:"foreignKey:SourceVMID"    json:"source_vm,omitempty"`
	TargetVM    VM          `gorm:"foreignKey:TargetVMID"    json:"target_vm,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// OrchestrationRun — tracks a single orchestration execution
// ─────────────────────────────────────────────────────────────────────────────

// OrchestrationRun records the execution of an orchestration operation
// (start/stop/restart/snapshot/clone) against an environment.
type OrchestrationRun struct {
	Base
	EnvironmentID uuid.UUID              `gorm:"type:uuid;not null;index"                 json:"environment_id"`
	Operation     OrchestrationOperation `gorm:"not null;size:32;index"                   json:"operation"`
	Status        OrchestrationRunStatus `gorm:"not null;default:'pending';size:32;index" json:"status"`
	Progress      int                    `gorm:"not null;default:0"                       json:"progress"` // 0–100
	TotalVMs      int                    `gorm:"not null;default:0"                       json:"total_vms"`
	CompletedVMs  int                    `gorm:"not null;default:0"                       json:"completed_vms"`
	FailedVMs     int                    `gorm:"not null;default:0"                       json:"failed_vms"`
	SkippedVMs    int                    `gorm:"not null;default:0"                       json:"skipped_vms"`
	ErrorMessage  string                 `gorm:"size:2048"                                json:"error_message,omitempty"`
	Payload       JSONMap                `gorm:"type:jsonb"                               json:"payload,omitempty"` // operation params
	Result        JSONMap                `gorm:"type:jsonb"                               json:"result,omitempty"`
	StartedAt     *time.Time             `gorm:"index"                                    json:"started_at,omitempty"`
	CompletedAt   *time.Time             `gorm:"index"                                    json:"completed_at,omitempty"`
	CreatedBy     *uuid.UUID             `gorm:"type:uuid;index"                          json:"created_by,omitempty"`

	// Relations
	Environment   Environment          `gorm:"foreignKey:EnvironmentID" json:"environment,omitempty"`
	CreatedByUser User                 `gorm:"foreignKey:CreatedBy"     json:"-"`
	Steps         []OrchestrationStep  `gorm:"foreignKey:RunID"         json:"steps,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// OrchestrationStep — per-VM step within a run
// ─────────────────────────────────────────────────────────────────────────────

// OrchestrationStepStatus tracks the state of a single VM step.
type OrchestrationStepStatus string

const (
	OrchestrationStepStatusPending   OrchestrationStepStatus = "pending"
	OrchestrationStepStatusRunning   OrchestrationStepStatus = "running"
	OrchestrationStepStatusCompleted OrchestrationStepStatus = "completed"
	OrchestrationStepStatusFailed    OrchestrationStepStatus = "failed"
	OrchestrationStepStatusSkipped   OrchestrationStepStatus = "skipped"
)

// OrchestrationStep is a single VM operation within an OrchestrationRun.
// Steps are ordered by ExecutionOrder and dispatched as individual tasks.
type OrchestrationStep struct {
	Base
	RunID          uuid.UUID               `gorm:"type:uuid;not null;index"                 json:"run_id"`
	VMID           uuid.UUID               `gorm:"type:uuid;not null;index"                 json:"vm_id"`
	ExecutionOrder int                     `gorm:"not null;default:0"                       json:"execution_order"`
	Status         OrchestrationStepStatus `gorm:"not null;default:'pending';size:32;index" json:"status"`
	TaskID         *uuid.UUID              `gorm:"type:uuid;index"                          json:"task_id,omitempty"`
	ErrorMessage   string                  `gorm:"size:2048"                                json:"error_message,omitempty"`
	StartedAt      *time.Time              `gorm:"index"                                    json:"started_at,omitempty"`
	CompletedAt    *time.Time              `gorm:"index"                                    json:"completed_at,omitempty"`

	// Relations
	Run  OrchestrationRun `gorm:"foreignKey:RunID"  json:"run,omitempty"`
	VM   VM               `gorm:"foreignKey:VMID"   json:"vm,omitempty"`
	Task *Task            `gorm:"foreignKey:TaskID" json:"task,omitempty"`
}
