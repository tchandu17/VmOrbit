package model

import (
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Schedule — cron-based or one-time scheduled operations
// ─────────────────────────────────────────────────────────────────────────────

// ScheduleType classifies the recurrence pattern.
type ScheduleType string

const (
	ScheduleTypeOnce    ScheduleType = "once"
	ScheduleTypeDaily   ScheduleType = "daily"
	ScheduleTypeWeekly  ScheduleType = "weekly"
	ScheduleTypeMonthly ScheduleType = "monthly"
	ScheduleTypeCron    ScheduleType = "cron"
)

// ScheduleStatus reflects the lifecycle state of a schedule.
type ScheduleStatus string

const (
	ScheduleStatusActive   ScheduleStatus = "active"
	ScheduleStatusPaused   ScheduleStatus = "paused"
	ScheduleStatusDisabled ScheduleStatus = "disabled"
	ScheduleStatusExpired  ScheduleStatus = "expired"
)

// ScheduleOperationType identifies the operation a schedule will trigger.
type ScheduleOperationType string

const (
	ScheduleOpInventorySync  ScheduleOperationType = "inventory.sync"
	ScheduleOpVMPowerOn      ScheduleOperationType = "vm.power_on"
	ScheduleOpVMPowerOff     ScheduleOperationType = "vm.power_off"
	ScheduleOpVMReboot       ScheduleOperationType = "vm.reboot"
	ScheduleOpVMSnapshot     ScheduleOperationType = "vm.snapshot"
	ScheduleOpVMBulkPowerOn  ScheduleOperationType = "vm.bulk.power_on"
	ScheduleOpVMBulkPowerOff ScheduleOperationType = "vm.bulk.power_off"
	ScheduleOpVMBulkReboot   ScheduleOperationType = "vm.bulk.reboot"
	ScheduleOpVMBulkSnapshot ScheduleOperationType = "vm.bulk.snapshot"
)

// ScheduleTargetType identifies what the schedule targets.
type ScheduleTargetType string

const (
	ScheduleTargetHypervisor ScheduleTargetType = "hypervisor"
	ScheduleTargetVM         ScheduleTargetType = "vm"
	ScheduleTargetTag        ScheduleTargetType = "tag"
)

// Schedule represents a recurring or one-time scheduled operation.
// CronExpression is the canonical schedule definition; for daily/weekly/monthly
// convenience types the engine converts them to cron expressions internally.
type Schedule struct {
	Base

	Name            string                `gorm:"not null;size:128;index"                json:"name"`
	Description     string                `gorm:"size:512"                               json:"description"`
	OperationType   ScheduleOperationType `gorm:"not null;size:64;index"                 json:"operation_type"`
	TargetType      ScheduleTargetType    `gorm:"not null;size:32"                       json:"target_type"`
	TargetIDs       StringArray           `gorm:"type:text[];not null;default:'{}'"      json:"target_ids"`
	ScheduleType    ScheduleType          `gorm:"not null;size:32"                       json:"schedule_type"`
	CronExpression  string                `gorm:"not null;size:128"                      json:"cron_expression"`
	Timezone        string                `gorm:"not null;size:64;default:'UTC'"         json:"timezone"`
	Enabled         bool                  `gorm:"not null;default:true;index"            json:"enabled"`
	Status          ScheduleStatus        `gorm:"not null;size:32;default:'active';index" json:"status"`
	LastRunAt       *time.Time            `gorm:"index"                                  json:"last_run_at,omitempty"`
	NextRunAt       *time.Time            `gorm:"index"                                  json:"next_run_at,omitempty"`
	LastTaskID      *uuid.UUID            `gorm:"type:uuid;index"                        json:"last_task_id,omitempty"`
	LastRunStatus   string                `gorm:"size:32"                                json:"last_run_status,omitempty"`
	RunCount        int                   `gorm:"not null;default:0"                     json:"run_count"`
	FailureCount    int                   `gorm:"not null;default:0"                     json:"failure_count"`
	MaxRuns         int                   `gorm:"not null;default:0"                     json:"max_runs"` // 0 = unlimited
	ExpiresAt       *time.Time            `gorm:"index"                                  json:"expires_at,omitempty"`
	Payload         JSONMap               `gorm:"type:jsonb"                             json:"payload,omitempty"`
	CreatedBy       *uuid.UUID            `gorm:"type:uuid;index"                        json:"created_by,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// ScheduleExecution — execution history for a schedule
// ─────────────────────────────────────────────────────────────────────────────

// ScheduleExecution records each time a schedule fires.
type ScheduleExecution struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt   time.Time  `gorm:"not null;index"                          json:"created_at"`
	ScheduleID  uuid.UUID  `gorm:"type:uuid;not null;index"                json:"schedule_id"`
	Schedule    *Schedule  `gorm:"foreignKey:ScheduleID"                   json:"schedule,omitempty"`
	TaskID      *uuid.UUID `gorm:"type:uuid;index"                         json:"task_id,omitempty"`
	Task        *Task      `gorm:"foreignKey:TaskID"                       json:"task,omitempty"`
	Status      string     `gorm:"not null;size:32;index"                  json:"status"` // triggered | skipped | failed
	ErrorMessage string    `gorm:"size:2048"                               json:"error_message,omitempty"`
	TriggeredAt time.Time  `gorm:"not null"                                json:"triggered_at"`
	CompletedAt *time.Time `gorm:"index"                                   json:"completed_at,omitempty"`
}
