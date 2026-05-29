package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// WebSocket event persistence
// ─────────────────────────────────────────────────────────────────────────────

// EventType classifies a WebSocket / domain event.
type EventType string

const (
	EventTypeVMStatusChanged       EventType = "vm.status_changed"
	EventTypeVMCreated             EventType = "vm.created"
	EventTypeVMDeleted             EventType = "vm.deleted"
	EventTypeTaskStatusChanged     EventType = "task.status_changed"
	EventTypeTaskCompleted         EventType = "task.completed"
	EventTypeTaskFailed            EventType = "task.failed"
	EventTypeHypervisorConnected   EventType = "hypervisor.connected"
	EventTypeHypervisorDisconnected EventType = "hypervisor.disconnected"
	EventTypeInventorySynced       EventType = "inventory.synced"
)

// WebSocketEvent is a persisted domain event that is also fanned out over
// WebSocket connections. Persisting events enables:
//   - replay for clients that reconnect after a gap
//   - audit trail of real-time state changes
//   - future migration to a proper event-sourcing store
//
// Rows are append-only. Retention policy (e.g. DELETE WHERE created_at < NOW() - INTERVAL '30 days')
// should be applied by a scheduled job or pg_partman.
type WebSocketEvent struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt time.Time `gorm:"not null;index"                          json:"created_at"`

	// Routing
	EventType    EventType  `gorm:"not null;index;size:64"                  json:"event_type"`
	Room         string     `gorm:"not null;index;size:128"                 json:"room"`         // WebSocket room name, e.g. "tasks", "vm:<id>"
	HypervisorID *uuid.UUID `gorm:"type:uuid;index"                         json:"hypervisor_id,omitempty"`
	ResourceID   *uuid.UUID `gorm:"type:uuid;index"                         json:"resource_id,omitempty"`

	// Payload — the full JSON body broadcast to subscribers
	Payload JSONMap `gorm:"type:jsonb;not null" json:"payload"`

	// Delivery tracking (best-effort; not a guarantee)
	DeliveredAt *time.Time `gorm:"index" json:"delivered_at,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (e *WebSocketEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
