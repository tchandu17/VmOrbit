package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// TaskStatus represents the lifecycle state of an async task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusRetrying  TaskStatus = "retrying"
	TaskStatusTimedOut  TaskStatus = "timed_out"
)

// TaskType categorises the operation being performed.
type TaskType string

const (
	TaskTypeVMPowerOn        TaskType = "vm.power_on"
	TaskTypeVMPowerOff       TaskType = "vm.power_off"
	TaskTypeVMReboot         TaskType = "vm.reboot"
	TaskTypeVMSuspend        TaskType = "vm.suspend"
	TaskTypeVMCreate         TaskType = "vm.create"
	TaskTypeVMDelete         TaskType = "vm.delete"
	TaskTypeVMClone          TaskType = "vm.clone"
	TaskTypeVMSnapshot       TaskType = "vm.snapshot"
	TaskTypeVMSnapshotDelete TaskType = "vm.snapshot.delete"
	TaskTypeVMRestore        TaskType = "vm.restore"
	TaskTypeVMMigrate        TaskType = "vm.migrate"
	TaskTypeInventorySync    TaskType = "inventory.sync"
	TaskTypeHypervisorSync   TaskType = "hypervisor.sync"

	// Bulk operations — parent tasks that fan out child tasks per VM.
	TaskTypeVMBulkPowerOn  TaskType = "vm.bulk.power_on"
	TaskTypeVMBulkPowerOff TaskType = "vm.bulk.power_off"
	TaskTypeVMBulkReboot   TaskType = "vm.bulk.reboot"
	TaskTypeVMBulkSnapshot TaskType = "vm.bulk.snapshot"

	// Template & provisioning operations.
	TaskTypeTemplateSync TaskType = "template.sync"
	TaskTypeVMCloneOp    TaskType = "vm.clone"
	TaskTypeVMProvision  TaskType = "vm.provision"

	// Environment orchestration operations.
	TaskTypeEnvStart    TaskType = "env.start"
	TaskTypeEnvStop     TaskType = "env.stop"
	TaskTypeEnvRestart  TaskType = "env.restart"
	TaskTypeEnvSnapshot TaskType = "env.snapshot"
	TaskTypeEnvClone    TaskType = "env.clone"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task
// ─────────────────────────────────────────────────────────────────────────────

// Task represents an async operation tracked in the database.
// ParentTaskID enables sub-task trees (e.g. a sync task spawning per-VM tasks).
// The composite index on (status, priority, scheduled_at) is the primary
// polling index used by the task engine worker loop.
type Task struct {
	Base
	Type         TaskType   `gorm:"not null;index;size:64"                  json:"type"`
	Status       TaskStatus `gorm:"not null;default:'pending';index;size:32" json:"status"`
	Priority     int        `gorm:"not null;default:5"                      json:"priority"` // 1 (highest) – 10 (lowest)
	Progress     int        `gorm:"not null;default:0"                      json:"progress"` // 0–100 percent
	Payload      JSONMap    `gorm:"type:jsonb"                              json:"payload"`
	Result       JSONMap    `gorm:"type:jsonb"                              json:"result,omitempty"`
	ErrorMessage string     `gorm:"size:2048"                               json:"error_message,omitempty"`
	RetryCount   int        `gorm:"not null;default:0"                      json:"retry_count"`
	MaxRetries   int        `gorm:"not null;default:3"                      json:"max_retries"`
	StartedAt    *time.Time `gorm:"index"                                   json:"started_at,omitempty"`
	CompletedAt  *time.Time `gorm:"index"                                   json:"completed_at,omitempty"`
	ScheduledAt  *time.Time `gorm:"index"                                   json:"scheduled_at,omitempty"`
	TimeoutAt    *time.Time `gorm:"index"                                   json:"timeout_at,omitempty"`

	// Ownership & scoping
	CreatedBy    *uuid.UUID `gorm:"type:uuid;index"  json:"created_by,omitempty"`
	HypervisorID *uuid.UUID `gorm:"type:uuid;index"  json:"hypervisor_id,omitempty"`
	VMID         *uuid.UUID `gorm:"type:uuid;index"  json:"vm_id,omitempty"`

	// Sub-task support: a nil ParentTaskID means this is a root task.
	ParentTaskID *uuid.UUID `gorm:"type:uuid;index"  json:"parent_task_id,omitempty"`

	// Relations
	CreatedByUser User   `gorm:"foreignKey:CreatedBy"    json:"-"`
	ParentTask    *Task  `gorm:"foreignKey:ParentTaskID" json:"parent_task,omitempty"`
	SubTasks      []Task `gorm:"foreignKey:ParentTaskID" json:"sub_tasks,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// TaskLog
// ─────────────────────────────────────────────────────────────────────────────

// TaskLogLevel classifies the severity of a task log entry.
type TaskLogLevel string

const (
	TaskLogLevelDebug TaskLogLevel = "debug"
	TaskLogLevelInfo  TaskLogLevel = "info"
	TaskLogLevelWarn  TaskLogLevel = "warn"
	TaskLogLevelError TaskLogLevel = "error"
)

// TaskLog is a single structured log line emitted by a task handler.
// Rows are append-only and scoped to a task. They are stored in Postgres
// for durability and mirrored to Redis (RPUSH / LTRIM) for low-latency
// streaming to WebSocket subscribers.
type TaskLog struct {
	ID        uuid.UUID    `gorm:"type:uuid;primaryKey"    json:"id"`
	CreatedAt time.Time    `gorm:"not null;index"          json:"created_at"`
	TaskID    uuid.UUID    `gorm:"type:uuid;not null;index" json:"task_id"`
	Level     TaskLogLevel `gorm:"not null;size:16"        json:"level"`
	Message   string       `gorm:"not null;size:4096"      json:"message"`
	Fields    JSONMap      `gorm:"type:jsonb"              json:"fields,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (l *TaskLog) BeforeCreate(_ *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
