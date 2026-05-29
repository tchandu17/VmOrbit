package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// AuditAction describes what happened.
type AuditAction string

const (
	AuditActionCreate  AuditAction = "create"
	AuditActionRead    AuditAction = "read"
	AuditActionUpdate  AuditAction = "update"
	AuditActionDelete  AuditAction = "delete"
	AuditActionLogin   AuditAction = "login"
	AuditActionLogout  AuditAction = "logout"
	AuditActionExecute AuditAction = "execute"
)

// ─────────────────────────────────────────────────────────────────────────────
// AuditLog
// ─────────────────────────────────────────────────────────────────────────────

// AuditLog is an immutable, append-only record of every significant platform
// action. There is intentionally no soft-delete (no deleted_at column) and no
// UpdatedAt — once written, a row must never change.
//
// Partitioning strategy: for high-volume deployments, range-partition this
// table by created_at (monthly). The migration file includes a comment showing
// how to do this with pg_partman or manual DDL.
type AuditLog struct {
	ID           uuid.UUID   `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt    time.Time   `gorm:"not null;index"                          json:"created_at"`

	// Actor
	UserID    *uuid.UUID `gorm:"type:uuid;index"                         json:"user_id,omitempty"` // nil for system actions
	Username  string     `gorm:"size:64"                                 json:"username"`

	// What happened
	Action      AuditAction `gorm:"not null;index;size:32"                  json:"action"`
	Resource    string      `gorm:"not null;index;size:64"                  json:"resource"`    // e.g. "vm"
	ResourceID  *uuid.UUID  `gorm:"type:uuid;index"                         json:"resource_id,omitempty"`
	Description string      `gorm:"size:1024"                               json:"description"`

	// Scoping — allows filtering audit logs by hypervisor without a JOIN
	HypervisorID *uuid.UUID `gorm:"type:uuid;index"                         json:"hypervisor_id,omitempty"`

	// Request context
	IPAddress string `gorm:"size:45"  json:"ip_address"`
	UserAgent string `gorm:"size:512" json:"user_agent"`
	RequestID string `gorm:"size:64;index" json:"request_id"`

	// Diff / outcome
	Changes      JSONMap `gorm:"type:jsonb" json:"changes,omitempty"`
	Metadata     JSONMap `gorm:"type:jsonb" json:"metadata,omitempty"`
	Success      bool    `gorm:"not null;default:true;index" json:"success"`
	ErrorMessage string  `gorm:"size:2048" json:"error_message,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (a *AuditLog) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
