package model

import "github.com/google/uuid"

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// ProvisioningJobStatus tracks the lifecycle of a provisioning job.
type ProvisioningJobStatus string

const (
	ProvisioningJobStatusPending   ProvisioningJobStatus = "pending"
	ProvisioningJobStatusRunning   ProvisioningJobStatus = "running"
	ProvisioningJobStatusCompleted ProvisioningJobStatus = "completed"
	ProvisioningJobStatusFailed    ProvisioningJobStatus = "failed"
	ProvisioningJobStatusCancelled ProvisioningJobStatus = "cancelled"
)

// ProvisioningJobType distinguishes clone-from-VM from provision-from-template.
type ProvisioningJobType string

const (
	ProvisioningJobTypeClone     ProvisioningJobType = "clone"
	ProvisioningJobTypeProvision ProvisioningJobType = "provision"
)

// ─────────────────────────────────────────────────────────────────────────────
// ProvisioningJob
// ─────────────────────────────────────────────────────────────────────────────

// ProvisioningJob records a clone or provision operation and its outcome.
// It is linked to the async task that performs the work so the UI can track
// progress via the existing task polling / WebSocket infrastructure.
type ProvisioningJob struct {
	Base

	// Classification
	Type   ProvisioningJobType   `gorm:"not null;size:32;index"  json:"type"`
	Status ProvisioningJobStatus `gorm:"not null;size:32;index;default:'pending'" json:"status"`

	// Source — exactly one of TemplateID or SourceVMID is set.
	TemplateID *uuid.UUID `gorm:"type:uuid;index" json:"template_id,omitempty"`
	SourceVMID *uuid.UUID `gorm:"type:uuid;index" json:"source_vm_id,omitempty"`

	// Target hypervisor
	HypervisorID uuid.UUID `gorm:"type:uuid;not null;index" json:"hypervisor_id"`

	// Desired VM configuration
	VMName      string  `gorm:"not null;size:256"  json:"vm_name"`
	CPUCount    int     `gorm:"not null;default:0" json:"cpu_count"`
	MemoryMB    int     `gorm:"not null;default:0" json:"memory_mb"`
	DiskGB      int     `gorm:"not null;default:0" json:"disk_gb"`
	NetworkName string  `gorm:"size:256"           json:"network_name"`
	DataStore   string  `gorm:"size:256"           json:"data_store"`
	Node        string  `gorm:"size:256"           json:"node"`
	Linked      bool    `gorm:"not null;default:false" json:"linked"`
	Tags        StringArray `gorm:"type:text[]"    json:"tags"`

	// Outcome
	ResultVMID   *uuid.UUID `gorm:"type:uuid;index" json:"result_vm_id,omitempty"`
	TaskID       *uuid.UUID `gorm:"type:uuid;index" json:"task_id,omitempty"`
	ErrorMessage string     `gorm:"size:2048"       json:"error_message,omitempty"`

	// Ownership
	CreatedBy *uuid.UUID `gorm:"type:uuid;index" json:"created_by,omitempty"`

	// Extra provider-specific options
	Metadata JSONMap `gorm:"type:jsonb" json:"metadata,omitempty"`

	// Relations
	Template   *VMTemplate `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	SourceVM   *VM         `gorm:"foreignKey:SourceVMID" json:"source_vm,omitempty"`
	Hypervisor Hypervisor  `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
	ResultVM   *VM         `gorm:"foreignKey:ResultVMID" json:"result_vm,omitempty"`
	Task       *Task       `gorm:"foreignKey:TaskID" json:"task,omitempty"`
}
