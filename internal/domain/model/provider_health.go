package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// ProviderHealth — per-hypervisor health snapshot
// ─────────────────────────────────────────────────────────────────────────────

// HealthStatus summarises the overall health of a provider.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// ProviderHealth is a rolling health snapshot for a registered hypervisor.
// One row per hypervisor — upserted on every health check cycle.
// Historical snapshots are stored in ProviderHealthHistory.
type ProviderHealth struct {
	ID           uuid.UUID    `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt    time.Time    `gorm:"not null;index"                          json:"created_at"`
	UpdatedAt    time.Time    `gorm:"not null"                                json:"updated_at"`
	HypervisorID uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex"          json:"hypervisor_id"`
	Status       HealthStatus `gorm:"not null;default:'unknown';index;size:32" json:"status"`

	// Connectivity
	Online          bool       `gorm:"not null;default:false"  json:"online"`
	LastSeenAt      *time.Time `gorm:"index"                   json:"last_seen_at,omitempty"`
	LastCheckAt     *time.Time `gorm:"index"                   json:"last_check_at,omitempty"`
	ConsecutiveFails int       `gorm:"not null;default:0"      json:"consecutive_fails"`

	// Latency (milliseconds)
	LatencyMs     float64 `gorm:"not null;default:0"  json:"latency_ms"`
	AvgLatencyMs  float64 `gorm:"not null;default:0"  json:"avg_latency_ms"`
	PeakLatencyMs float64 `gorm:"not null;default:0"  json:"peak_latency_ms"`

	// Sync health
	LastSyncAt      *time.Time `gorm:"index"              json:"last_sync_at,omitempty"`
	LastSyncStatus  string     `gorm:"size:32"            json:"last_sync_status"` // "success" | "failed" | "running"
	SyncFailures24h int        `gorm:"not null;default:0" json:"sync_failures_24h"`
	VMCount         int        `gorm:"not null;default:0" json:"vm_count"`

	// Task health (rolling 24 h window)
	TasksTotal24h   int     `gorm:"not null;default:0"  json:"tasks_total_24h"`
	TasksFailed24h  int     `gorm:"not null;default:0"  json:"tasks_failed_24h"`
	TaskFailureRate float64 `gorm:"not null;default:0"  json:"task_failure_rate"` // 0–1

	// Auth failures (rolling 24 h window)
	AuthFailures24h int `gorm:"not null;default:0" json:"auth_failures_24h"`

	// Inventory freshness
	InventoryAgeMinutes int  `gorm:"not null;default:0"     json:"inventory_age_minutes"`
	InventoryStale      bool `gorm:"not null;default:false" json:"inventory_stale"`

	// Computed health score 0–100
	HealthScore int `gorm:"not null;default:0" json:"health_score"`

	// Relation
	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (p *ProviderHealth) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ProviderHealthHistory — time-series health snapshots
// ─────────────────────────────────────────────────────────────────────────────

// ProviderHealthHistory stores one row per health check cycle per hypervisor.
// Used for latency graphs and trend analysis. Rows are append-only.
type ProviderHealthHistory struct {
	ID           uuid.UUID    `gorm:"type:uuid;primaryKey"           json:"id"`
	CreatedAt    time.Time    `gorm:"not null;index"                 json:"created_at"`
	HypervisorID uuid.UUID    `gorm:"type:uuid;not null;index"       json:"hypervisor_id"`
	Status       HealthStatus `gorm:"not null;size:32"               json:"status"`
	Online       bool         `gorm:"not null;default:false"         json:"online"`
	LatencyMs    float64      `gorm:"not null;default:0"             json:"latency_ms"`
	HealthScore  int          `gorm:"not null;default:0"             json:"health_score"`
	ErrorMessage string       `gorm:"size:512"                       json:"error_message,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (p *ProviderHealthHistory) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
