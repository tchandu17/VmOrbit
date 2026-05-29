package model

import (
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// User
// ─────────────────────────────────────────────────────────────────────────────

// User represents a platform user with RBAC support.
type User struct {
	Base
	Email        string     `gorm:"uniqueIndex;not null;size:320"  json:"email"`
	Username     string     `gorm:"uniqueIndex;not null;size:64"   json:"username"`
	PasswordHash string     `gorm:"not null"                       json:"-"`
	FirstName    string     `gorm:"size:128"                       json:"first_name"`
	LastName     string     `gorm:"size:128"                       json:"last_name"`
	IsActive     bool       `gorm:"not null;default:true;index"    json:"is_active"`
	IsVerified   bool       `gorm:"not null;default:false"         json:"is_verified"`
	LastLoginAt  *time.Time `                                      json:"last_login_at,omitempty"`

	// RBAC — join table: user_roles
	Roles []Role `gorm:"many2many:user_roles;" json:"roles,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// RBAC
// ─────────────────────────────────────────────────────────────────────────────

// Role represents a named permission set.
type Role struct {
	Base
	Name        string       `gorm:"uniqueIndex;not null;size:64" json:"name"`
	Description string       `gorm:"size:512"                     json:"description"`

	// join table: role_permissions
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	Users       []User       `gorm:"many2many:user_roles;"       json:"-"`
}

// Permission is a fine-grained action on a resource.
// The composite unique index prevents duplicate (resource, action) pairs.
type Permission struct {
	Base
	Resource string `gorm:"not null;size:64;uniqueIndex:uidx_permission_resource_action" json:"resource"` // e.g. "vm"
	Action   string `gorm:"not null;size:64;uniqueIndex:uidx_permission_resource_action" json:"action"`   // e.g. "read"

	Roles []Role `gorm:"many2many:role_permissions;" json:"-"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Auth tokens
// ─────────────────────────────────────────────────────────────────────────────

// RefreshToken stores JWT refresh tokens for rotation.
// Tokens are hashed before storage; the raw value is never persisted.
type RefreshToken struct {
	Base
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"       json:"user_id"`
	TokenHash string    `gorm:"uniqueIndex;not null"           json:"-"` // SHA-256 of the raw token
	ExpiresAt time.Time `gorm:"not null;index"                 json:"expires_at"`
	Revoked   bool      `gorm:"not null;default:false;index"   json:"revoked"`
	UserAgent string    `gorm:"size:512"                       json:"user_agent"`
	IPAddress string    `gorm:"size:45"                        json:"ip_address"` // IPv6 max = 45 chars

	User User `gorm:"foreignKey:UserID" json:"-"`
}
