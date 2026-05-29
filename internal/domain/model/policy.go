package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// PolicyType categorises what the policy governs.
type PolicyType string

const (
	PolicyTypeVM          PolicyType = "vm"
	PolicyTypeEnvironment PolicyType = "environment"
	PolicyTypeProvider    PolicyType = "provider"
	PolicyTypeTask        PolicyType = "task"
	PolicyTypeUser        PolicyType = "user"
)

// PolicyEffect is the outcome when a policy matches.
type PolicyEffect string

const (
	PolicyEffectAllow           PolicyEffect = "allow"
	PolicyEffectDeny            PolicyEffect = "deny"
	PolicyEffectRequireApproval PolicyEffect = "require_approval"
	PolicyEffectRequireSnapshot PolicyEffect = "require_snapshot"
	PolicyEffectRequireJustification PolicyEffect = "require_justification"
)

// PolicyConditionType identifies the kind of condition being evaluated.
type PolicyConditionType string

const (
	PolicyConditionVMTag          PolicyConditionType = "vm_tag"
	PolicyConditionEnvironment    PolicyConditionType = "environment"
	PolicyConditionProvider       PolicyConditionType = "provider"
	PolicyConditionUserRole       PolicyConditionType = "user_role"
	PolicyConditionOperation      PolicyConditionType = "operation"
	PolicyConditionMaintenanceWindow PolicyConditionType = "maintenance_window"
	PolicyConditionTimeSchedule   PolicyConditionType = "time_schedule"
	PolicyConditionVMName         PolicyConditionType = "vm_name"
	PolicyConditionHypervisor     PolicyConditionType = "hypervisor"
	PolicyConditionBulkSize       PolicyConditionType = "bulk_size"
)

// PolicyConditionOperator defines how the condition value is compared.
type PolicyConditionOperator string

const (
	PolicyConditionOpEquals      PolicyConditionOperator = "equals"
	PolicyConditionOpNotEquals   PolicyConditionOperator = "not_equals"
	PolicyConditionOpContains    PolicyConditionOperator = "contains"
	PolicyConditionOpIn          PolicyConditionOperator = "in"
	PolicyConditionOpNotIn       PolicyConditionOperator = "not_in"
	PolicyConditionOpGreaterThan PolicyConditionOperator = "greater_than"
	PolicyConditionOpLessThan    PolicyConditionOperator = "less_than"
	PolicyConditionOpMatches     PolicyConditionOperator = "matches" // regex
)

// PolicyViolationStatus tracks the outcome of a violation record.
type PolicyViolationStatus string

const (
	PolicyViolationStatusBlocked  PolicyViolationStatus = "blocked"
	PolicyViolationStatusOverridden PolicyViolationStatus = "overridden"
	PolicyViolationStatusPending  PolicyViolationStatus = "pending_approval"
)

// ─────────────────────────────────────────────────────────────────────────────
// Policy
// ─────────────────────────────────────────────────────────────────────────────

// Policy is a governance rule that intercepts infrastructure operations.
// When all conditions match, the effect is applied.
type Policy struct {
	Base
	Name        string     `gorm:"not null;size:128;uniqueIndex" json:"name"`
	Description string     `gorm:"size:512"                     json:"description"`
	Type        PolicyType `gorm:"not null;size:32;index"        json:"type"`
	Effect      PolicyEffect `gorm:"not null;size:32"             json:"effect"`
	Priority    int        `gorm:"not null;default:100"          json:"priority"` // lower = higher priority
	Enabled     bool       `gorm:"not null;default:true;index"   json:"enabled"`

	// Operations this policy applies to (comma-separated task types or "*" for all).
	// Stored as text[] for efficient GIN indexing.
	Operations StringArray `gorm:"type:text[];not null;default:'{}'" json:"operations"`

	// Optional: approval configuration when effect = require_approval
	ApprovalConfig JSONMap `gorm:"type:jsonb" json:"approval_config,omitempty"`

	// Metadata for additional policy-specific configuration
	Metadata JSONMap `gorm:"type:jsonb" json:"metadata,omitempty"`

	// Relations
	Conditions  []PolicyCondition  `gorm:"foreignKey:PolicyID" json:"conditions,omitempty"`
	Assignments []PolicyAssignment `gorm:"foreignKey:PolicyID" json:"assignments,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// PolicyCondition
// ─────────────────────────────────────────────────────────────────────────────

// PolicyCondition is a single predicate that must be satisfied for the policy
// to fire. All conditions on a policy are ANDed together.
type PolicyCondition struct {
	ID        uuid.UUID               `gorm:"type:uuid;primaryKey"             json:"id"`
	CreatedAt time.Time               `gorm:"not null"                         json:"created_at"`
	PolicyID  uuid.UUID               `gorm:"type:uuid;not null;index"         json:"policy_id"`
	Type      PolicyConditionType     `gorm:"not null;size:64"                 json:"type"`
	Operator  PolicyConditionOperator `gorm:"not null;size:32"                 json:"operator"`
	// Value is the comparison target. For "in"/"not_in" operators, use a JSON array string.
	Value     string                  `gorm:"not null;size:1024"               json:"value"`
	// Negate inverts the condition result.
	Negate    bool                    `gorm:"not null;default:false"           json:"negate"`

	Policy Policy `gorm:"foreignKey:PolicyID" json:"-"`
}

// BeforeCreate sets a UUID primary key.
func (c *PolicyCondition) BeforeCreate(_ *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// PolicyAssignment
// ─────────────────────────────────────────────────────────────────────────────

// PolicyAssignmentTargetType identifies what the policy is assigned to.
type PolicyAssignmentTargetType string

const (
	PolicyAssignmentTargetGlobal      PolicyAssignmentTargetType = "global"
	PolicyAssignmentTargetHypervisor  PolicyAssignmentTargetType = "hypervisor"
	PolicyAssignmentTargetEnvironment PolicyAssignmentTargetType = "environment"
	PolicyAssignmentTargetVM          PolicyAssignmentTargetType = "vm"
	PolicyAssignmentTargetTag         PolicyAssignmentTargetType = "tag"
	PolicyAssignmentTargetRole        PolicyAssignmentTargetType = "role"
)

// PolicyAssignment links a policy to a specific target scope.
// A global assignment (target_type = "global") applies to all operations.
type PolicyAssignment struct {
	ID         uuid.UUID                  `gorm:"type:uuid;primaryKey"             json:"id"`
	CreatedAt  time.Time                  `gorm:"not null"                         json:"created_at"`
	PolicyID   uuid.UUID                  `gorm:"type:uuid;not null;index"         json:"policy_id"`
	TargetType PolicyAssignmentTargetType `gorm:"not null;size:32;index"           json:"target_type"`
	// TargetID is the UUID of the target resource. Empty for global assignments.
	TargetID   string                     `gorm:"size:36;index"                    json:"target_id,omitempty"`
	CreatedBy  *uuid.UUID                 `gorm:"type:uuid;index"                  json:"created_by,omitempty"`

	Policy Policy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
}

// BeforeCreate sets a UUID primary key.
func (a *PolicyAssignment) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// PolicyViolation
// ─────────────────────────────────────────────────────────────────────────────

// PolicyViolation is an immutable record of every policy enforcement event.
type PolicyViolation struct {
	ID          uuid.UUID             `gorm:"type:uuid;primaryKey"             json:"id"`
	CreatedAt   time.Time             `gorm:"not null;index"                   json:"created_at"`
	PolicyID    uuid.UUID             `gorm:"type:uuid;not null;index"         json:"policy_id"`
	PolicyName  string                `gorm:"not null;size:128"                json:"policy_name"`
	Effect      PolicyEffect          `gorm:"not null;size:32"                 json:"effect"`
	Status      PolicyViolationStatus `gorm:"not null;size:32;index"           json:"status"`

	// What triggered the violation
	Operation    string     `gorm:"not null;size:64;index"           json:"operation"`
	ResourceType string     `gorm:"not null;size:64"                 json:"resource_type"`
	ResourceID   string     `gorm:"size:36;index"                    json:"resource_id"`
	ResourceName string     `gorm:"size:256"                         json:"resource_name"`

	// Who triggered it
	UserID    *uuid.UUID `gorm:"type:uuid;index"                  json:"user_id,omitempty"`
	Username  string     `gorm:"size:64"                          json:"username"`

	// Linked approval request (when effect = require_approval)
	ApprovalRequestID *uuid.UUID `gorm:"type:uuid;index"              json:"approval_request_id,omitempty"`

	// Justification provided by the user (when effect = require_justification)
	Justification string `gorm:"size:2048"                        json:"justification,omitempty"`

	// Additional context
	Metadata JSONMap `gorm:"type:jsonb"                       json:"metadata,omitempty"`

	Policy Policy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
}

// BeforeCreate sets a UUID primary key.
func (v *PolicyViolation) BeforeCreate(_ *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	return nil
}
