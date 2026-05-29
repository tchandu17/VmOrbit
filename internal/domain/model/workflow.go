package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Workflow — rule-based automation
// ─────────────────────────────────────────────────────────────────────────────

// WorkflowTriggerType identifies what event fires a workflow.
type WorkflowTriggerType string

const (
	WorkflowTriggerSchedule            WorkflowTriggerType = "schedule"
	WorkflowTriggerProviderDisconnected WorkflowTriggerType = "provider_disconnected"
	WorkflowTriggerSyncFailure         WorkflowTriggerType = "sync_failure"
	WorkflowTriggerTaskFailure         WorkflowTriggerType = "task_failure"
	WorkflowTriggerVMStateChange       WorkflowTriggerType = "vm_state_change"
	WorkflowTriggerManual              WorkflowTriggerType = "manual"
)

// WorkflowStatus reflects the lifecycle state of a workflow definition.
type WorkflowStatus string

const (
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusPaused   WorkflowStatus = "paused"
	WorkflowStatusDisabled WorkflowStatus = "disabled"
)

// WorkflowRunStatus reflects the execution state of a workflow run.
type WorkflowRunStatus string

const (
	WorkflowRunStatusPending   WorkflowRunStatus = "pending"
	WorkflowRunStatusRunning   WorkflowRunStatus = "running"
	WorkflowRunStatusCompleted WorkflowRunStatus = "completed"
	WorkflowRunStatusFailed    WorkflowRunStatus = "failed"
	WorkflowRunStatusCancelled WorkflowRunStatus = "cancelled"
)

// WorkflowActionType identifies the action a workflow step performs.
type WorkflowActionType string

const (
	WorkflowActionCreateSnapshot  WorkflowActionType = "create_snapshot"
	WorkflowActionPowerOn         WorkflowActionType = "power_on"
	WorkflowActionPowerOff        WorkflowActionType = "power_off"
	WorkflowActionReboot          WorkflowActionType = "reboot"
	WorkflowActionSendNotification WorkflowActionType = "send_notification"
	WorkflowActionTriggerSync     WorkflowActionType = "trigger_sync"
	WorkflowActionWebhook         WorkflowActionType = "webhook"
	WorkflowActionDelay           WorkflowActionType = "delay"
)

// ─────────────────────────────────────────────────────────────────────────────
// Workflow
// ─────────────────────────────────────────────────────────────────────────────

// Workflow is a rule-based automation definition.
// It consists of a trigger, optional conditions, and an ordered list of actions.
type Workflow struct {
	Base

	Name        string              `gorm:"not null;size:128;index"                json:"name"`
	Description string              `gorm:"size:512"                               json:"description"`
	Enabled     bool                `gorm:"not null;default:true;index"            json:"enabled"`
	Status      WorkflowStatus      `gorm:"not null;size:32;default:'active';index" json:"status"`
	TriggerType WorkflowTriggerType `gorm:"not null;size:64;index"                 json:"trigger_type"`

	// TriggerConfig holds trigger-specific parameters.
	// schedule: { "cron_expression": "0 2 * * *", "timezone": "UTC" }
	// vm_state_change: { "from_state": "running", "to_state": "stopped" }
	// provider_disconnected: { "provider": "vmware" }
	TriggerConfig JSONMap `gorm:"type:jsonb" json:"trigger_config,omitempty"`

	// Conditions is a list of conditions that must ALL be true for the workflow to run.
	// Each condition: { "field": "vm.tags", "operator": "contains", "value": "production" }
	Conditions JSONMap `gorm:"type:jsonb" json:"conditions,omitempty"`

	// ContinueOnError controls whether the workflow continues if an action fails.
	ContinueOnError bool `gorm:"not null;default:false" json:"continue_on_error"`

	// MaxConcurrentRuns limits how many runs can be active simultaneously.
	MaxConcurrentRuns int `gorm:"not null;default:1" json:"max_concurrent_runs"`

	// RunCount tracks total executions.
	RunCount     int        `gorm:"not null;default:0"  json:"run_count"`
	FailureCount int        `gorm:"not null;default:0"  json:"failure_count"`
	LastRunAt    *time.Time `gorm:"index"               json:"last_run_at,omitempty"`
	LastRunStatus string    `gorm:"size:32"             json:"last_run_status,omitempty"`

	CreatedBy *uuid.UUID `gorm:"type:uuid;index" json:"created_by,omitempty"`

	// Relations
	Actions []WorkflowAction `gorm:"foreignKey:WorkflowID;constraint:OnDelete:CASCADE" json:"actions,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// WorkflowAction — a single step in a workflow
// ─────────────────────────────────────────────────────────────────────────────

// WorkflowAction is an ordered step in a workflow.
type WorkflowAction struct {
	Base

	WorkflowID  uuid.UUID          `gorm:"type:uuid;not null;index"  json:"workflow_id"`
	Order       int                `gorm:"not null;default:0;index"  json:"order"`
	ActionType  WorkflowActionType `gorm:"not null;size:64"          json:"action_type"`
	Name        string             `gorm:"size:128"                  json:"name"`
	Description string             `gorm:"size:512"                  json:"description"`

	// Config holds action-specific parameters.
	// create_snapshot: { "name_template": "auto-{{date}}", "description": "...", "memory": false }
	// power_on/off/reboot: { "target_type": "vm|tag", "target_ids": [...] }
	// send_notification: { "channel_id": "...", "message": "..." }
	// trigger_sync: { "hypervisor_id": "..." }
	// webhook: { "url": "...", "method": "POST", "headers": {...}, "body": "..." }
	// delay: { "seconds": 30 }
	Config JSONMap `gorm:"type:jsonb" json:"config,omitempty"`

	// RetryCount is how many times to retry this action on failure.
	RetryCount int `gorm:"not null;default:0" json:"retry_count"`

	// TimeoutSeconds is the maximum time to wait for this action to complete.
	TimeoutSeconds int `gorm:"not null;default:300" json:"timeout_seconds"`

	// ContinueOnError overrides the workflow-level setting for this action.
	ContinueOnError *bool `gorm:"" json:"continue_on_error,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// WorkflowRun — a single execution of a workflow
// ─────────────────────────────────────────────────────────────────────────────

// WorkflowRun records a single execution of a workflow.
type WorkflowRun struct {
	ID          uuid.UUID         `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt   time.Time         `gorm:"not null;index"                          json:"created_at"`
	WorkflowID  uuid.UUID         `gorm:"type:uuid;not null;index"                json:"workflow_id"`
	Workflow    *Workflow         `gorm:"foreignKey:WorkflowID"                   json:"workflow,omitempty"`
	Status      WorkflowRunStatus `gorm:"not null;size:32;index"                  json:"status"`
	TriggerType WorkflowTriggerType `gorm:"not null;size:64"                      json:"trigger_type"`
	TriggerData JSONMap           `gorm:"type:jsonb"                              json:"trigger_data,omitempty"`
	StartedAt   *time.Time        `gorm:"index"                                   json:"started_at,omitempty"`
	CompletedAt *time.Time        `gorm:"index"                                   json:"completed_at,omitempty"`
	ErrorMessage string           `gorm:"size:2048"                               json:"error_message,omitempty"`
	ActionsRun  int               `gorm:"not null;default:0"                      json:"actions_run"`
	ActionsFailed int             `gorm:"not null;default:0"                      json:"actions_failed"`
	Logs        JSONMap           `gorm:"type:jsonb"                              json:"logs,omitempty"`
	TriggeredBy *uuid.UUID        `gorm:"type:uuid;index"                         json:"triggered_by,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (r *WorkflowRun) BeforeCreate(_ *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	return nil
}
