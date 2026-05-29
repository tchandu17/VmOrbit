package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConsoleSessionStatus reflects the lifecycle state of a console session.
type ConsoleSessionStatus string

const (
	ConsoleSessionStatusActive  ConsoleSessionStatus = "active"
	ConsoleSessionStatusExpired ConsoleSessionStatus = "expired"
	ConsoleSessionStatusRevoked ConsoleSessionStatus = "revoked"
)

// ConsoleSession records a short-lived console access grant.
// It is created when a user requests console access and expires automatically.
// The session_token is a random UUID used as a lookup key — it is NOT the
// provider ticket (which is stored in provider_ticket).
type ConsoleSession struct {
	ID             uuid.UUID            `gorm:"type:uuid;primaryKey"                         json:"id"`
	VMID           uuid.UUID            `gorm:"type:uuid;not null;index"                     json:"vm_id"`
	HypervisorID   uuid.UUID            `gorm:"type:uuid;not null;index"                     json:"hypervisor_id"`
	UserID         *uuid.UUID           `gorm:"type:uuid;index"                              json:"user_id,omitempty"`
	Provider       ProviderType         `gorm:"not null;size:32"                             json:"provider"`
	ConsoleType    string               `gorm:"not null;size:32"                             json:"console_type"` // webmks | novnc | vnc
	SessionToken   string               `gorm:"not null;uniqueIndex;size:64"                 json:"session_token"`
	ProviderTicket string               `gorm:"not null;size:2048"                           json:"provider_ticket"`
	ConsoleURL     string               `gorm:"not null;size:2048"                           json:"console_url"`
	Status         ConsoleSessionStatus `gorm:"not null;default:'active';index;size:16"      json:"status"`
	ExpiresAt      time.Time            `gorm:"not null;index"                               json:"expires_at"`
	CreatedAt      time.Time            `gorm:"not null"                                     json:"created_at"`
	// Extra holds provider-specific fields (e.g. ssl_thumbprint, wss_url, node, vmid).
	Extra JSONMap `gorm:"type:jsonb" json:"extra,omitempty"`
}

// IsExpired returns true if the session has passed its expiry time.
func (s *ConsoleSession) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// BeforeCreate sets a UUID primary key before inserting.
func (s *ConsoleSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
