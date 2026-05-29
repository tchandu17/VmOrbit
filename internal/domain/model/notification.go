package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Notification Channel
// ─────────────────────────────────────────────────────────────────────────────

// NotificationChannelType identifies the delivery mechanism.
type NotificationChannelType string

const (
	NotificationChannelEmail   NotificationChannelType = "email"
	NotificationChannelSlack   NotificationChannelType = "slack"
	NotificationChannelWebhook NotificationChannelType = "webhook"
)

// NotificationChannel stores the configuration for a delivery endpoint.
// Sensitive fields (SMTP password, Slack token) are stored in Config JSONB
// and should be encrypted at rest in production.
type NotificationChannel struct {
	Base

	Name        string                  `gorm:"not null;size:128;uniqueIndex" json:"name"`
	Type        NotificationChannelType `gorm:"not null;size:32;index"        json:"type"`
	Description string                  `gorm:"size:512"                      json:"description,omitempty"`
	Enabled     bool                    `gorm:"not null;default:true"         json:"enabled"`

	// Type-specific configuration stored as JSONB.
	// Email:   { "host", "port", "username", "password", "from", "to": [...], "tls" }
	// Slack:   { "webhook_url", "channel", "username", "icon_emoji" }
	// Webhook: { "url", "method", "headers": {...}, "secret" }
	Config JSONMap `gorm:"type:jsonb;not null" json:"config"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Notification Rule
// ─────────────────────────────────────────────────────────────────────────────

// NotificationRule maps event criteria to a delivery channel.
// When a PlatformEvent matches all active filters, the rule triggers delivery.
type NotificationRule struct {
	Base

	Name        string `gorm:"not null;size:128"  json:"name"`
	Description string `gorm:"size:512"           json:"description,omitempty"`
	Enabled     bool   `gorm:"not null;default:true" json:"enabled"`

	// Channel to deliver to
	ChannelID uuid.UUID            `gorm:"type:uuid;not null;index" json:"channel_id"`
	Channel   *NotificationChannel `gorm:"foreignKey:ChannelID"     json:"channel,omitempty"`

	// Filters — empty/nil means "match all"
	// EventTypes is a comma-separated list of PlatformEventType values.
	// Stored as text[] in Postgres for efficient GIN indexing.
	EventTypes  StringArray           `gorm:"type:text[];not null;default:'{}'" json:"event_types"`
	Severities  StringArray           `gorm:"type:text[];not null;default:'{}'" json:"severities"`
	Providers   StringArray           `gorm:"type:text[];not null;default:'{}'" json:"providers"`

	// Throttle: minimum seconds between deliveries for this rule.
	// 0 = no throttle.
	ThrottleSeconds int `gorm:"not null;default:0" json:"throttle_seconds"`

	// LastTriggeredAt tracks throttle state.
	LastTriggeredAt *time.Time `gorm:"index" json:"last_triggered_at,omitempty"`
}

// Matches returns true if the event satisfies all non-empty filters on the rule.
func (r *NotificationRule) Matches(event *PlatformEvent) bool {
	if !r.Enabled {
		return false
	}

	// Event type filter
	if len(r.EventTypes) > 0 {
		found := false
		for _, et := range r.EventTypes {
			if et == string(event.EventType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Severity filter
	if len(r.Severities) > 0 {
		found := false
		for _, s := range r.Severities {
			if s == string(event.Severity) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Provider filter
	if len(r.Providers) > 0 && event.Provider != "" {
		found := false
		for _, p := range r.Providers {
			if p == event.Provider {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// IsThrottled returns true if the rule has been triggered too recently.
func (r *NotificationRule) IsThrottled() bool {
	if r.ThrottleSeconds <= 0 || r.LastTriggeredAt == nil {
		return false
	}
	return time.Since(*r.LastTriggeredAt) < time.Duration(r.ThrottleSeconds)*time.Second
}

// ─────────────────────────────────────────────────────────────────────────────
// Notification History
// ─────────────────────────────────────────────────────────────────────────────

// NotificationStatus tracks the delivery outcome.
type NotificationStatus string

const (
	NotificationStatusPending  NotificationStatus = "pending"
	NotificationStatusDelivered NotificationStatus = "delivered"
	NotificationStatusFailed   NotificationStatus = "failed"
	NotificationStatusThrottled NotificationStatus = "throttled"
)

// NotificationHistory is an append-only delivery log.
// One row per delivery attempt.
type NotificationHistory struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt time.Time `gorm:"not null;index"                          json:"created_at"`

	// Linkage
	RuleID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"rule_id"`
	Rule      *NotificationRule `gorm:"foreignKey:RuleID"   json:"rule,omitempty"`
	ChannelID uuid.UUID  `gorm:"type:uuid;not null;index" json:"channel_id"`
	Channel   *NotificationChannel `gorm:"foreignKey:ChannelID" json:"channel,omitempty"`
	EventID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"event_id"`
	Event     *PlatformEvent `gorm:"foreignKey:EventID"   json:"event,omitempty"`

	// Delivery outcome
	Status       NotificationStatus `gorm:"not null;size:16;index" json:"status"`
	ErrorMessage string             `gorm:"size:2048"              json:"error_message,omitempty"`
	AttemptCount int                `gorm:"not null;default:1"     json:"attempt_count"`
	DeliveredAt  *time.Time         `gorm:"index"                  json:"delivered_at,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (n *NotificationHistory) BeforeCreate(_ *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	return nil
}
