package port

import (
	"context"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Repository interfaces
// ─────────────────────────────────────────────────────────────────────────────

// InfrastructureMetricsRepository defines persistence for infrastructure metrics.
type InfrastructureMetricsRepository interface {
	// Insert appends a new metrics snapshot.
	Insert(ctx context.Context, m *model.InfrastructureMetrics) error
	// ListSince returns snapshots collected after the given time, ordered oldest-first.
	ListSince(ctx context.Context, since time.Time, limit int) ([]model.InfrastructureMetrics, error)
	// Latest returns the most recent snapshot.
	Latest(ctx context.Context) (*model.InfrastructureMetrics, error)
}

// CapacityHistoryRepository defines persistence for per-hypervisor capacity history.
type CapacityHistoryRepository interface {
	// Insert appends a new capacity snapshot.
	Insert(ctx context.Context, h *model.CapacityHistory) error
	// ListByHypervisor returns snapshots for a hypervisor after the given time.
	ListByHypervisor(ctx context.Context, hypervisorID string, since time.Time, limit int) ([]model.CapacityHistory, error)
	// ListAllSince returns snapshots for all hypervisors after the given time.
	ListAllSince(ctx context.Context, since time.Time, limit int) ([]model.CapacityHistory, error)
}

// ProviderCapacityRepository defines persistence for live provider capacity.
type ProviderCapacityRepository interface {
	// Upsert inserts or updates the capacity record for a hypervisor.
	Upsert(ctx context.Context, c *model.ProviderCapacity) error
	// GetByHypervisor returns the current capacity for a hypervisor.
	GetByHypervisor(ctx context.Context, hypervisorID string) (*model.ProviderCapacity, error)
	// List returns capacity records for all hypervisors.
	List(ctx context.Context) ([]model.ProviderCapacity, error)
}

// VMUsageStatsRepository defines persistence for per-VM usage statistics.
type VMUsageStatsRepository interface {
	// Upsert inserts or updates usage stats for a VM.
	Upsert(ctx context.Context, s *model.VMUsageStats) error
	// GetByVM returns usage stats for a VM.
	GetByVM(ctx context.Context, vmID string) (*model.VMUsageStats, error)
	// List returns usage stats for all VMs, optionally filtered.
	List(ctx context.Context, filter VMUsageStatsFilter) ([]model.VMUsageStats, error)
}

// VMUsageStatsFilter narrows VM usage stats queries.
type VMUsageStatsFilter struct {
	HypervisorID string
	IsOversized  *bool
	IsIdle       *bool
	HasStaleSnaps *bool
}

// OptimizationRecommendationRepository defines persistence for recommendations.
type OptimizationRecommendationRepository interface {
	// Upsert inserts or updates a recommendation by fingerprint.
	Upsert(ctx context.Context, r *model.OptimizationRecommendation) error
	// GetByID returns a recommendation by ID.
	GetByID(ctx context.Context, id string) (*model.OptimizationRecommendation, error)
	// List returns recommendations with optional filters.
	List(ctx context.Context, filter RecommendationFilter, page Page) (*PageResult[model.OptimizationRecommendation], error)
	// UpdateStatus changes the status of a recommendation.
	UpdateStatus(ctx context.Context, id string, status model.RecommendationStatus, changedBy *string, note string) error
	// CountByStatus returns counts grouped by status.
	CountByStatus(ctx context.Context) (map[string]int64, error)
	// CountBySeverity returns active recommendation counts grouped by severity.
	CountBySeverity(ctx context.Context) (map[string]int64, error)
}

// RecommendationFilter narrows recommendation list queries.
type RecommendationFilter struct {
	Type         string
	Severity     string
	Status       string
	HypervisorID string
	VMID         string
}

// RecommendationHistoryRepository defines persistence for recommendation history.
type RecommendationHistoryRepository interface {
	// Create appends a history entry.
	Create(ctx context.Context, h *model.RecommendationHistory) error
	// ListByRecommendation returns history for a recommendation.
	ListByRecommendation(ctx context.Context, recommendationID string) ([]model.RecommendationHistory, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Service interface
// ─────────────────────────────────────────────────────────────────────────────

// AnalyticsService provides infrastructure analytics, capacity planning, and
// optimization recommendations.
type AnalyticsService interface {
	// ── Capacity ──────────────────────────────────────────────────────────────
	// GetCapacitySummary returns the current infrastructure capacity overview.
	GetCapacitySummary(ctx context.Context) (*CapacitySummary, error)
	// GetCapacityTrends returns time-series capacity data for trend analysis.
	GetCapacityTrends(ctx context.Context, req CapacityTrendsRequest) (*CapacityTrends, error)
	// GetProviderCapacity returns capacity details per hypervisor.
	GetProviderCapacity(ctx context.Context) ([]model.ProviderCapacity, error)

	// ── Recommendations ───────────────────────────────────────────────────────
	// GetRecommendations returns paginated optimization recommendations.
	GetRecommendations(ctx context.Context, filter RecommendationFilter, page Page) (*PageResult[model.OptimizationRecommendation], error)
	// DismissRecommendation marks a recommendation as dismissed.
	DismissRecommendation(ctx context.Context, id string, userID string, note string) error
	// ResolveRecommendation marks a recommendation as resolved.
	ResolveRecommendation(ctx context.Context, id string, userID string) error
	// GetRecommendationSummary returns counts by severity and type.
	GetRecommendationSummary(ctx context.Context) (*RecommendationSummary, error)

	// ── Forecasting ───────────────────────────────────────────────────────────
	// GetForecasts returns capacity forecasts for all hypervisors.
	GetForecasts(ctx context.Context) (*ForecastReport, error)

	// ── Collection (called by the analytics engine) ───────────────────────────
	// CollectMetrics runs a full metrics collection cycle.
	CollectMetrics(ctx context.Context) error
	// RunOptimizationEngine analyses the current state and generates recommendations.
	RunOptimizationEngine(ctx context.Context) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Response types
// ─────────────────────────────────────────────────────────────────────────────

// CapacitySummary is the top-level infrastructure capacity overview.
type CapacitySummary struct {
	// Totals
	TotalHypervisors  int   `json:"total_hypervisors"`
	TotalVMs          int   `json:"total_vms"`
	RunningVMs        int   `json:"running_vms"`
	StoppedVMs        int   `json:"stopped_vms"`
	TotalCPUCores     int   `json:"total_cpu_cores"`
	TotalMemoryMB     int64 `json:"total_memory_mb"`
	TotalDiskGB       int64 `json:"total_disk_gb"`
	TotalSnapshots    int   `json:"total_snapshots"`
	TotalEnvironments int   `json:"total_environments"`

	// Utilisation
	CPUUtilisationPct     float64 `json:"cpu_utilisation_pct"`
	MemoryUtilisationPct  float64 `json:"memory_utilisation_pct"`
	StorageUtilisationPct float64 `json:"storage_utilisation_pct"`

	// Storage (GB)
	TotalStorageGB float64 `json:"total_storage_gb"`
	UsedStorageGB  float64 `json:"used_storage_gb"`
	FreeStorageGB  float64 `json:"free_storage_gb"`

	// VM density
	VMDensity float64 `json:"vm_density"`

	// Task stats (24 h)
	TasksCompleted24h int `json:"tasks_completed_24h"`
	TasksFailed24h    int `json:"tasks_failed_24h"`

	// Recommendation counts
	CriticalRecommendations int `json:"critical_recommendations"`
	WarningRecommendations  int `json:"warning_recommendations"`
	InfoRecommendations     int `json:"info_recommendations"`

	// Timestamp
	CollectedAt time.Time `json:"collected_at"`
}

// CapacityTrendsRequest carries parameters for trend queries.
type CapacityTrendsRequest struct {
	HypervisorID string    // empty = all hypervisors
	Since        time.Time // start of the time window
	Until        time.Time // end of the time window (zero = now)
	Granularity  string    // "hour" | "day" | "week"
}

// CapacityTrends holds time-series data for charts.
type CapacityTrends struct {
	// Infrastructure-level time series
	VMGrowth      []TimeSeriesPoint `json:"vm_growth"`
	CPUTrend      []TimeSeriesPoint `json:"cpu_trend"`
	MemoryTrend   []TimeSeriesPoint `json:"memory_trend"`
	StorageTrend  []TimeSeriesPoint `json:"storage_trend"`
	SnapshotTrend []TimeSeriesPoint `json:"snapshot_trend"`
	TaskTrend     []TimeSeriesPoint `json:"task_trend"`

	// Per-provider breakdown (hypervisor_id → series)
	ProviderVMGrowth map[string][]TimeSeriesPoint `json:"provider_vm_growth"`
}

// TimeSeriesPoint is a single data point in a time series.
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Label     string    `json:"label,omitempty"`
}

// RecommendationSummary holds aggregated recommendation counts.
type RecommendationSummary struct {
	TotalActive    int64            `json:"total_active"`
	TotalDismissed int64            `json:"total_dismissed"`
	TotalResolved  int64            `json:"total_resolved"`
	BySeverity     map[string]int64 `json:"by_severity"`
	ByType         map[string]int64 `json:"by_type"`
}

// ForecastReport holds capacity forecasts for all hypervisors.
type ForecastReport struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Forecasts   []HypervisorForecast `json:"forecasts"`
	GlobalRisks []ForecastRisk     `json:"global_risks"`
}

// HypervisorForecast holds capacity forecasts for a single hypervisor.
type HypervisorForecast struct {
	HypervisorID   string  `json:"hypervisor_id"`
	HypervisorName string  `json:"hypervisor_name"`
	Provider       string  `json:"provider"`

	// Storage forecast
	StorageExhaustionDays  *int    `json:"storage_exhaustion_days,omitempty"`
	StorageGrowthRateGBDay float64 `json:"storage_growth_rate_gb_day"`
	CurrentStorageUsedPct  float64 `json:"current_storage_used_pct"`

	// VM growth forecast
	VMGrowthRatePerDay float64 `json:"vm_growth_rate_per_day"`
	ProjectedVMs30Days int     `json:"projected_vms_30_days"`

	// Snapshot growth
	SnapshotGrowthRatePerDay float64 `json:"snapshot_growth_rate_per_day"`
	SnapshotRisk             string  `json:"snapshot_risk"` // "low" | "medium" | "high"

	// Risks
	Risks []ForecastRisk `json:"risks"`
}

// ForecastRisk describes a predicted capacity risk.
type ForecastRisk struct {
	Type        string `json:"type"`        // "storage_exhaustion" | "vm_saturation" | "snapshot_growth"
	Severity    string `json:"severity"`    // "info" | "warning" | "critical"
	Description string `json:"description"`
	DaysUntil   *int   `json:"days_until,omitempty"`
}
