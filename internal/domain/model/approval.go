package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// ApprovalStatus tracks the lifecycle of an approval request.
type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "pending"
	ApprovalStatusApproved  ApprovalStatus = "approved"
	ApprovalStatusRejected  ApprovalStatus = "rejected"
	ApprovalStatusExpired   ApprovalStatus = "expired"
	ApprovalStatusEscalated ApprovalStatus = "escalated"
	ApprovalStatusCancelled ApprovalStatus = "cancelled"
)

// ApprovalStepStatus tracks the state of a single step in the approval chain.
type ApprovalStepStatus string

const (
	ApprovalStepStatusPending  ApprovalStepStatus = "pending"
	ApprovalStepStatusApproved ApprovalStepStatus = "approved"
	ApprovalStepStatusRejected ApprovalStepStatus = "rejected"
	ApprovalStepStatusSkipped  ApprovalStepStatus = "skipped"
)

// ApprovalHistoryAction records what happened at each history point.
type ApprovalHistoryAction string

const (
	ApprovalHistoryActionCreated   ApprovalHistoryAction = "created"
	ApprovalHistoryActionApproved  ApprovalHistoryAction = "approved"
	ApprovalHistoryActionRejected  ApprovalHistoryAction = "rejected"
	ApprovalHistoryActionEscalated ApprovalHistoryAction = "escalated"
	ApprovalHistoryActionExpired   ApprovalHistoryAction = "expired"
	ApprovalHistoryActionCancelled ApprovalHistoryAction = "cancelled"
	ApprovalHistoryActionCommented ApprovalHistoryAction = "commented"
)

// ─────────────────────────────────────────────────────────────────────────────
// ApprovalRequest
// ─────────────────────────────────────────────────────────────────────────────

// ApprovalRequest is created when a policy with effect=require_approval fires.
// The original operation is held until the request is approved or rejected.
type ApprovalRequest struct {
	Base
	// Policy that triggered this request
	PolicyID   uuid.UUID `gorm:"type:uuid;not null;index"         json:"policy_id"`
	PolicyName string    `gorm:"not null;size:128"                json:"policy_name"`

	// The operation being requested
	Operation    string  `gorm:"not null;size:64;index"           json:"operation"`
	ResourceType string  `gorm:"not null;size:64"                 json:"resource_type"`
	ResourceID   string  `gorm:"size:36;index"                    json:"resource_id"`
	ResourceName string  `gorm:"size:256"                         json:"resource_name"`

	// Requester
	RequesterID   uuid.UUID `gorm:"type:uuid;not null;index"     json:"requester_id"`
	RequesterName string    `gorm:"not null;size:128"            json:"requester_name"`

	// Justification provided by the requester
	Justification string `gorm:"size:2048"                        json:"justification"`

	// Lifecycle
	Status    ApprovalStatus `gorm:"not null;size:32;index;default:'pending'" json:"status"`
	ExpiresAt *time.Time     `gorm:"index"                            json:"expires_at,omitempty"`
	ResolvedAt *time.Time    `gorm:"index"                            json:"resolved_at,omitempty"`

	// Escalation
	EscalatedAt  *time.Time `gorm:"index"                            json:"escalated_at,omitempty"`
	EscalatedTo  *uuid.UUID `gorm:"type:uuid;index"                  json:"escalated_to,omitempty"`

	// The task payload to execute upon approval
	OperationPayload JSONMap `gorm:"type:jsonb"                   json:"operation_payload,omitempty"`

	// Additional context
	Metadata JSONMap `gorm:"type:jsonb"                       json:"metadata,omitempty"`

	// Relations
	Steps   []ApprovalStep   `gorm:"foreignKey:RequestID" json:"steps,omitempty"`
	History []ApprovalHistory `gorm:"foreignKey:RequestID" json:"history,omitempty"`
	Policy  Policy            `gorm:"foreignKey:PolicyID"  json:"policy,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// ApprovalStep
// ─────────────────────────────────────────────────────────────────────────────

// ApprovalStep represents one level in a multi-level approval chain.
// Steps are ordered by StepOrder and processed sequentially.
type ApprovalStep struct {
	ID        uuid.UUID          `gorm:"type:uuid;primaryKey"             json:"id"`
	CreatedAt time.Time          `gorm:"not null"                         json:"created_at"`
	RequestID uuid.UUID          `gorm:"type:uuid;not null;index"         json:"request_id"`
	StepOrder int                `gorm:"not null;default:1"               json:"step_order"`
	Status    ApprovalStepStatus `gorm:"not null;size:32;default:'pending'" json:"status"`

	// Who can approve this step — either a specific user or a role
	ApproverID   *uuid.UUID `gorm:"type:uuid;index"                  json:"approver_id,omitempty"`
	ApproverRole string     `gorm:"size:64;index"                    json:"approver_role,omitempty"`
	ApproverName string     `gorm:"size:128"                         json:"approver_name,omitempty"`

	// Resolution
	ResolvedBy   *uuid.UUID `gorm:"type:uuid;index"                  json:"resolved_by,omitempty"`
	ResolvedByName string   `gorm:"size:128"                         json:"resolved_by_name,omitempty"`
	ResolvedAt   *time.Time `gorm:"index"                            json:"resolved_at,omitempty"`
	Comment      string     `gorm:"size:2048"                        json:"comment,omitempty"`

	Request ApprovalRequest `gorm:"foreignKey:RequestID" json:"-"`
}

// BeforeCreate sets a UUID primary key.
func (s *ApprovalStep) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ApprovalHistory
// ─────────────────────────────────────────────────────────────────────────────

// ApprovalHistory is an append-only audit trail for an approval request.
type ApprovalHistory struct {
	ID        uuid.UUID             `gorm:"type:uuid;primaryKey"             json:"id"`
	CreatedAt time.Time             `gorm:"not null;index"                   json:"created_at"`
	RequestID uuid.UUID             `gorm:"type:uuid;not null;index"         json:"request_id"`
	Action    ApprovalHistoryAction `gorm:"not null;size:32"                 json:"action"`
	ActorID   *uuid.UUID            `gorm:"type:uuid;index"                  json:"actor_id,omitempty"`
	ActorName string                `gorm:"size:128"                         json:"actor_name"`
	Comment   string                `gorm:"size:2048"                        json:"comment,omitempty"`
	Metadata  JSONMap               `gorm:"type:jsonb"                       json:"metadata,omitempty"`

	Request ApprovalRequest `gorm:"foreignKey:RequestID" json:"-"`
}

// BeforeCreate sets a UUID primary key.
func (h *ApprovalHistory) BeforeCreate(_ *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}
	return nil
}
