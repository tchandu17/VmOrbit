package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────────────────────────────────────
// InfrastructureMetrics — point-in-time snapshot of infrastructure utilisation
// ─────────────────────────────────────────────────────────────────────────────

// InfrastructureMetrics stores a periodic snapshot of the entire platform's
// resource utilisation. One row is inserted per collection interval (default 15 min).
// Rows are append-only — never updated or soft-deleted.
type InfrastructureMetrics struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"     json:"id"`
	CollectedAt time.Time `gorm:"not null;index"           json:"collected_at"`

	// Totals across all hypervisors
	TotalVMs        int     `gorm:"not null;default:0" json:"total_vms"`
	RunningVMs      int     `gorm:"not null;default:0" json:"running_vms"`
	StoppedVMs      int     `gorm:"not null;default:0" json:"stopped_vms"`
	PoweredOffVMs   int     `gorm:"not null;default:0" json:"powered_off_vms"`
	TotalCPUCores   int     `gorm:"not null;default:0" json:"total_cpu_cores"`
	TotalMemoryMB   int64   `gorm:"not null;default:0" json:"total_memory_mb"`
	TotalDiskGB     int64   `gorm:"not null;default:0" json:"total_disk_gb"`
	TotalSnapshots  int     `gorm:"not null;default:0" json:"total_snapshots"`
	TotalHypervisors int    `gorm:"not null;default:0" json:"total_hypervisors"`
	TotalEnvironments int   `gorm:"not null;default:0" json:"total_environments"`

	// Utilisation percentages (0–100)
	CPUUtilisationPct    float64 `gorm:"not null;default:0" json:"cpu_utilisation_pct"`
	MemoryUtilisationPct float64 `gorm:"not null;default:0" json:"memory_utilisation_pct"`
	StorageUtilisationPct float64 `gorm:"not null;default:0" json:"storage_utilisation_pct"`

	// VM density (running VMs per hypervisor)
	VMDensity float64 `gorm:"not null;default:0" json:"vm_density"`

	// Task execution stats (rolling 24 h window at collection time)
	TasksCompleted24h int `gorm:"not null;default:0" json:"tasks_completed_24h"`
	TasksFailed24h    int `gorm:"not null;default:0" json:"tasks_failed_24h"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (m *InfrastructureMetrics) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.CollectedAt.IsZero() {
		m.CollectedAt = time.Now().UTC()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// CapacityHistory — per-hypervisor capacity time-series
// ─────────────────────────────────────────────────────────────────────────────

// CapacityHistory stores per-hypervisor capacity snapshots over time.
// Used for trend analysis and forecasting.
type CapacityHistory struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"           json:"id"`
	CollectedAt  time.Time `gorm:"not null;index"                 json:"collected_at"`
	HypervisorID uuid.UUID `gorm:"type:uuid;not null;index"       json:"hypervisor_id"`

	// VM counts
	TotalVMs   int `gorm:"not null;default:0" json:"total_vms"`
	RunningVMs int `gorm:"not null;default:0" json:"running_vms"`

	// Compute
	TotalCPUCores int   `gorm:"not null;default:0" json:"total_cpu_cores"`
	TotalMemoryMB int64 `gorm:"not null;default:0" json:"total_memory_mb"`

	// Storage (aggregated across all datastores for this hypervisor)
	TotalStorageGB float64 `gorm:"not null;default:0" json:"total_storage_gb"`
	UsedStorageGB  float64 `gorm:"not null;default:0" json:"used_storage_gb"`
	FreeStorageGB  float64 `gorm:"not null;default:0" json:"free_storage_gb"`

	// Snapshot count
	SnapshotCount int `gorm:"not null;default:0" json:"snapshot_count"`

	// Relation
	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (c *CapacityHistory) BeforeCreate(_ *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CollectedAt.IsZero() {
		c.CollectedAt = time.Now().UTC()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ProviderCapacity — current capacity summary per hypervisor (upserted)
// ─────────────────────────────────────────────────────────────────────────────

// ProviderCapacity is a live capacity summary for a single hypervisor.
// One row per hypervisor — upserted on every collection cycle.
type ProviderCapacity struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"                    json:"id"`
	UpdatedAt    time.Time `gorm:"not null"                                json:"updated_at"`
	HypervisorID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"          json:"hypervisor_id"`

	// VM counts
	TotalVMs   int `gorm:"not null;default:0" json:"total_vms"`
	RunningVMs int `gorm:"not null;default:0" json:"running_vms"`
	StoppedVMs int `gorm:"not null;default:0" json:"stopped_vms"`

	// Compute
	TotalCPUCores int   `gorm:"not null;default:0" json:"total_cpu_cores"`
	TotalMemoryMB int64 `gorm:"not null;default:0" json:"total_memory_mb"`

	// Storage
	TotalStorageGB float64 `gorm:"not null;default:0" json:"total_storage_gb"`
	UsedStorageGB  float64 `gorm:"not null;default:0" json:"used_storage_gb"`
	FreeStorageGB  float64 `gorm:"not null;default:0" json:"free_storage_gb"`
	StorageUsedPct float64 `gorm:"not null;default:0" json:"storage_used_pct"`

	// Snapshot count
	SnapshotCount int `gorm:"not null;default:0" json:"snapshot_count"`

	// Relation
	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (p *ProviderCapacity) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// VMUsageStats — per-VM usage statistics (upserted)
// ─────────────────────────────────────────────────────────────────────────────

// VMUsageStats stores per-VM usage statistics for optimization analysis.
// One row per VM — upserted on every collection cycle.
type VMUsageStats struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"           json:"id"`
	UpdatedAt time.Time `gorm:"not null"                       json:"updated_at"`
	VMID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"vm_id"`

	// Resource allocation
	CPUCount int   `gorm:"not null;default:0" json:"cpu_count"`
	MemoryMB int   `gorm:"not null;default:0" json:"memory_mb"`
	DiskGB   int   `gorm:"not null;default:0" json:"disk_gb"`

	// Snapshot count for this VM
	SnapshotCount int `gorm:"not null;default:0" json:"snapshot_count"`

	// Days since last power-on (for idle detection)
	DaysSinceLastPowerOn int `gorm:"not null;default:0" json:"days_since_last_power_on"`

	// Flags set by the optimization engine
	IsOversized    bool `gorm:"not null;default:false" json:"is_oversized"`
	IsIdle         bool `gorm:"not null;default:false" json:"is_idle"`
	HasStaleSnaps  bool `gorm:"not null;default:false" json:"has_stale_snaps"`

	// Relation
	VM VM `gorm:"foreignKey:VMID" json:"vm,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (v *VMUsageStats) BeforeCreate(_ *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// OptimizationRecommendation — actionable infrastructure improvement suggestions
// ─────────────────────────────────────────────────────────────────────────────

// RecommendationType classifies the kind of optimization.
type RecommendationType string

const (
	RecommendationOversizedVM       RecommendationType = "oversized_vm"
	RecommendationIdleVM            RecommendationType = "idle_vm"
	RecommendationStaleSnapshot     RecommendationType = "stale_snapshot"
	RecommendationUnderutilizedHost RecommendationType = "underutilized_host"
	RecommendationOvercommittedHost RecommendationType = "overcommitted_host"
	RecommendationPoweredOffStaleVM RecommendationType = "powered_off_stale_vm"
	RecommendationOrphanedResource  RecommendationType = "orphaned_resource"
	RecommendationSnapshotGrowth    RecommendationType = "snapshot_growth"
	RecommendationStorageExhaustion RecommendationType = "storage_exhaustion"
)

// RecommendationSeverity classifies urgency.
type RecommendationSeverity string

const (
	RecommendationSeverityInfo     RecommendationSeverity = "info"
	RecommendationSeverityWarning  RecommendationSeverity = "warning"
	RecommendationSeverityCritical RecommendationSeverity = "critical"
)

// RecommendationStatus tracks the lifecycle of a recommendation.
type RecommendationStatus string

const (
	RecommendationStatusActive    RecommendationStatus = "active"
	RecommendationStatusDismissed RecommendationStatus = "dismissed"
	RecommendationStatusResolved  RecommendationStatus = "resolved"
)

// OptimizationRecommendation is an actionable suggestion generated by the
// optimization engine. Recommendations are upserted by a fingerprint key so
// the same issue is not duplicated across collection cycles.
type OptimizationRecommendation struct {
	ID          uuid.UUID              `gorm:"type:uuid;primaryKey"                    json:"id"`
	CreatedAt   time.Time              `gorm:"not null;index"                          json:"created_at"`
	UpdatedAt   time.Time              `gorm:"not null"                                json:"updated_at"`

	// Fingerprint uniquely identifies this recommendation so it can be upserted.
	// Format: "<type>:<resource_id>"
	Fingerprint string `gorm:"not null;uniqueIndex;size:256" json:"fingerprint"`

	// Classification
	Type     RecommendationType     `gorm:"not null;size:64;index"  json:"type"`
	Severity RecommendationSeverity `gorm:"not null;size:16;index"  json:"severity"`
	Status   RecommendationStatus   `gorm:"not null;size:16;index;default:'active'" json:"status"`

	// Scoping — at most one of these is set
	HypervisorID *uuid.UUID `gorm:"type:uuid;index" json:"hypervisor_id,omitempty"`
	VMID         *uuid.UUID `gorm:"type:uuid;index" json:"vm_id,omitempty"`

	// Human-readable content
	Title       string `gorm:"not null;size:256"  json:"title"`
	Description string `gorm:"not null;size:2048" json:"description"`
	Action      string `gorm:"size:512"           json:"action"` // suggested remediation

	// Scoring (0–100; higher = more impactful)
	Score int `gorm:"not null;default:0" json:"score"`

	// Estimated savings / impact
	EstimatedSavingsGB  float64 `gorm:"not null;default:0" json:"estimated_savings_gb"`
	EstimatedSavingsCPU int     `gorm:"not null;default:0" json:"estimated_savings_cpu"`
	EstimatedSavingsMB  int     `gorm:"not null;default:0" json:"estimated_savings_mb"`

	// Lifecycle
	DismissedAt *time.Time `gorm:"index" json:"dismissed_at,omitempty"`
	ResolvedAt  *time.Time `gorm:"index" json:"resolved_at,omitempty"`
	DismissedBy *uuid.UUID `gorm:"type:uuid" json:"dismissed_by,omitempty"`

	// Extra context
	Metadata JSONMap `gorm:"type:jsonb" json:"metadata,omitempty"`

	// Relations
	Hypervisor *Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
	VM         *VM         `gorm:"foreignKey:VMID"         json:"vm,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (r *OptimizationRecommendation) BeforeCreate(_ *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RecommendationHistory — audit trail for recommendation lifecycle changes
// ─────────────────────────────────────────────────────────────────────────────

// RecommendationHistory records every status change on a recommendation.
type RecommendationHistory struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"           json:"id"`
	CreatedAt        time.Time `gorm:"not null;index"                 json:"created_at"`
	RecommendationID uuid.UUID `gorm:"type:uuid;not null;index"       json:"recommendation_id"`
	OldStatus        string    `gorm:"not null;size:16"               json:"old_status"`
	NewStatus        string    `gorm:"not null;size:16"               json:"new_status"`
	ChangedBy        *uuid.UUID `gorm:"type:uuid"                     json:"changed_by,omitempty"`
	Note             string    `gorm:"size:512"                       json:"note,omitempty"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (h *RecommendationHistory) BeforeCreate(_ *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}
	return nil
}
