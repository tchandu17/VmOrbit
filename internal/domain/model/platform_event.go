package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Platform Event — persisted domain events for the event activity feed
// ─────────────────────────────────────────────────────────────────────────────

// PlatformEventType classifies a platform-level event.
type PlatformEventType string

const (
	// Provider lifecycle
	PlatformEventProviderConnected    PlatformEventType = "provider_connected"
	PlatformEventProviderDisconnected PlatformEventType = "provider_disconnected"

	// Inventory sync
	PlatformEventSyncCompleted PlatformEventType = "sync_completed"
	PlatformEventSyncFailed    PlatformEventType = "sync_failed"

	// VM power operations
	PlatformEventVMPowerOnSuccess  PlatformEventType = "vm_poweron_success"
	PlatformEventVMPowerOnFailed   PlatformEventType = "vm_poweron_failed"
	PlatformEventVMPowerOffSuccess PlatformEventType = "vm_poweroff_success"
	PlatformEventVMPowerOffFailed  PlatformEventType = "vm_poweroff_failed"
	PlatformEventVMRebootSuccess   PlatformEventType = "vm_reboot_success"
	PlatformEventVMRebootFailed    PlatformEventType = "vm_reboot_failed"

	// Snapshots
	PlatformEventSnapshotCreated PlatformEventType = "snapshot_created"
	PlatformEventSnapshotFailed  PlatformEventType = "snapshot_failed"
	PlatformEventSnapshotDeleted PlatformEventType = "snapshot_deleted"
	PlatformEventSnapshotReverted PlatformEventType = "snapshot_reverted"

	// Tasks
	PlatformEventTaskFailed PlatformEventType = "task_failed"

	// Bulk operations
	PlatformEventBulkOperationFailed PlatformEventType = "bulk_operation_failed"

	// Security
	PlatformEventLoginFailed       PlatformEventType = "login_failed"
	PlatformEventPermissionDenied  PlatformEventType = "permission_denied"

	// Scheduling & Automation
	PlatformEventScheduleFired     PlatformEventType = "schedule_fired"
	PlatformEventScheduleFailed    PlatformEventType = "schedule_failed"
	PlatformEventWorkflowExecuted  PlatformEventType = "workflow_executed"
	PlatformEventWorkflowFailed    PlatformEventType = "workflow_failed"
)

// PlatformEventSeverity classifies the urgency of an event.
type PlatformEventSeverity string

const (
	PlatformEventSeverityInfo     PlatformEventSeverity = "info"
	PlatformEventSeverityWarning  PlatformEventSeverity = "warning"
	PlatformEventSeverityCritical PlatformEventSeverity = "critical"
)

// PlatformEvent is a persisted, queryable record of a significant platform
// activity. It is append-only — rows are never updated or soft-deleted.
//
// Events are published via the EventDispatcher which:
//  1. Persists the row to Postgres.
//  2. Publishes to the in-memory EventBus so the WebSocket hub fans it out.
//  3. Enqueues notification delivery for matching rules.
type PlatformEvent struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt time.Time `gorm:"not null;index"                          json:"created_at"`

	// Classification
	EventType    PlatformEventType     `gorm:"not null;index;size:64"  json:"event_type"`
	Severity     PlatformEventSeverity `gorm:"not null;index;size:16"  json:"severity"`

	// Scoping
	Provider     string     `gorm:"size:32;index"           json:"provider,omitempty"`   // vmware | proxmox | esxi | ""
	ResourceType string     `gorm:"size:64;index"           json:"resource_type,omitempty"` // vm | hypervisor | snapshot | task
	ResourceID   *uuid.UUID `gorm:"type:uuid;index"         json:"resource_id,omitempty"`
	HypervisorID *uuid.UUID `gorm:"type:uuid;index"         json:"hypervisor_id,omitempty"`

	// Human-readable summary
	Message  string  `gorm:"not null;size:1024"      json:"message"`
	Metadata JSONMap `gorm:"type:jsonb"              json:"metadata,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (e *PlatformEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	return nil
}

// SeverityForEventType returns the default severity for a given event type.
func SeverityForEventType(t PlatformEventType) PlatformEventSeverity {
	switch t {
	case PlatformEventProviderDisconnected,
		PlatformEventSyncFailed,
		PlatformEventVMPowerOnFailed,
		PlatformEventVMPowerOffFailed,
		PlatformEventVMRebootFailed,
		PlatformEventSnapshotFailed,
		PlatformEventTaskFailed,
		PlatformEventBulkOperationFailed:
		return PlatformEventSeverityCritical
	case PlatformEventLoginFailed,
		PlatformEventPermissionDenied:
		return PlatformEventSeverityWarning
	default:
		return PlatformEventSeverityInfo
	}
}
