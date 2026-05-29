package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ─────────────────────────────────────────────────────────────────────────────
// InfrastructureMetricsRepo
// ─────────────────────────────────────────────────────────────────────────────

type InfrastructureMetricsRepo struct{ db *gorm.DB }

func NewInfrastructureMetricsRepo(db *gorm.DB) *InfrastructureMetricsRepo {
	return &InfrastructureMetricsRepo{db: db}
}

func (r *InfrastructureMetricsRepo) Insert(ctx context.Context, m *model.InfrastructureMetrics) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *InfrastructureMetricsRepo) ListSince(ctx context.Context, since time.Time, limit int) ([]model.InfrastructureMetrics, error) {
	var items []model.InfrastructureMetrics
	err := r.db.WithContext(ctx).
		Where("collected_at >= ?", since).
		Order("collected_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *InfrastructureMetricsRepo) Latest(ctx context.Context) (*model.InfrastructureMetrics, error) {
	var m model.InfrastructureMetrics
	if err := r.db.WithContext(ctx).Order("collected_at DESC").First(&m).Error; err != nil {
		return nil, fmt.Errorf("no metrics found: %w", err)
	}
	return &m, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// CapacityHistoryRepo
// ─────────────────────────────────────────────────────────────────────────────

type CapacityHistoryRepo struct{ db *gorm.DB }

func NewCapacityHistoryRepo(db *gorm.DB) *CapacityHistoryRepo {
	return &CapacityHistoryRepo{db: db}
}

func (r *CapacityHistoryRepo) Insert(ctx context.Context, h *model.CapacityHistory) error {
	return r.db.WithContext(ctx).Create(h).Error
}

func (r *CapacityHistoryRepo) ListByHypervisor(ctx context.Context, hypervisorID string, since time.Time, limit int) ([]model.CapacityHistory, error) {
	var items []model.CapacityHistory
	err := r.db.WithContext(ctx).
		Where("hypervisor_id = ? AND collected_at >= ?", hypervisorID, since).
		Order("collected_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *CapacityHistoryRepo) ListAllSince(ctx context.Context, since time.Time, limit int) ([]model.CapacityHistory, error) {
	var items []model.CapacityHistory
	err := r.db.WithContext(ctx).
		Where("collected_at >= ?", since).
		Order("collected_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// ─────────────────────────────────────────────────────────────────────────────
// ProviderCapacityRepo
// ─────────────────────────────────────────────────────────────────────────────

type ProviderCapacityRepo struct{ db *gorm.DB }

func NewProviderCapacityRepo(db *gorm.DB) *ProviderCapacityRepo {
	return &ProviderCapacityRepo{db: db}
}

func (r *ProviderCapacityRepo) Upsert(ctx context.Context, c *model.ProviderCapacity) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hypervisor_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"updated_at",
				"total_vms", "running_vms", "stopped_vms",
				"total_cpu_cores", "total_memory_mb",
				"total_storage_gb", "used_storage_gb", "free_storage_gb", "storage_used_pct",
				"snapshot_count",
			}),
		}).
		Create(c).Error
}

func (r *ProviderCapacityRepo) GetByHypervisor(ctx context.Context, hypervisorID string) (*model.ProviderCapacity, error) {
	var c model.ProviderCapacity
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Where("hypervisor_id = ?", hypervisorID).
		First(&c).Error; err != nil {
		return nil, fmt.Errorf("provider capacity not found: %w", err)
	}
	return &c, nil
}

func (r *ProviderCapacityRepo) List(ctx context.Context) ([]model.ProviderCapacity, error) {
	var items []model.ProviderCapacity
	err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Find(&items).Error
	return items, err
}

// ─────────────────────────────────────────────────────────────────────────────
// VMUsageStatsRepo
// ─────────────────────────────────────────────────────────────────────────────

type VMUsageStatsRepo struct{ db *gorm.DB }

func NewVMUsageStatsRepo(db *gorm.DB) *VMUsageStatsRepo {
	return &VMUsageStatsRepo{db: db}
}

func (r *VMUsageStatsRepo) Upsert(ctx context.Context, s *model.VMUsageStats) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "vm_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"updated_at",
				"cpu_count", "memory_mb", "disk_gb",
				"snapshot_count", "days_since_last_power_on",
				"is_oversized", "is_idle", "has_stale_snaps",
			}),
		}).
		Create(s).Error
}

func (r *VMUsageStatsRepo) GetByVM(ctx context.Context, vmID string) (*model.VMUsageStats, error) {
	var s model.VMUsageStats
	if err := r.db.WithContext(ctx).Where("vm_id = ?", vmID).First(&s).Error; err != nil {
		return nil, fmt.Errorf("vm usage stats not found: %w", err)
	}
	return &s, nil
}

func (r *VMUsageStatsRepo) List(ctx context.Context, filter port.VMUsageStatsFilter) ([]model.VMUsageStats, error) {
	var items []model.VMUsageStats
	q := r.db.WithContext(ctx).Preload("VM")

	if filter.HypervisorID != "" {
		q = q.Joins("JOIN vms ON vms.id = vm_usage_stats.vm_id").
			Where("vms.hypervisor_id = ?", filter.HypervisorID)
	}
	if filter.IsOversized != nil {
		q = q.Where("is_oversized = ?", *filter.IsOversized)
	}
	if filter.IsIdle != nil {
		q = q.Where("is_idle = ?", *filter.IsIdle)
	}
	if filter.HasStaleSnaps != nil {
		q = q.Where("has_stale_snaps = ?", *filter.HasStaleSnaps)
	}

	err := q.Find(&items).Error
	return items, err
}

// ─────────────────────────────────────────────────────────────────────────────
// OptimizationRecommendationRepo
// ─────────────────────────────────────────────────────────────────────────────

type OptimizationRecommendationRepo struct{ db *gorm.DB }

func NewOptimizationRecommendationRepo(db *gorm.DB) *OptimizationRecommendationRepo {
	return &OptimizationRecommendationRepo{db: db}
}

func (r *OptimizationRecommendationRepo) Upsert(ctx context.Context, rec *model.OptimizationRecommendation) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "fingerprint"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"updated_at",
				"type", "severity",
				"title", "description", "action",
				"score",
				"estimated_savings_gb", "estimated_savings_cpu", "estimated_savings_mb",
				"metadata",
				// Do NOT overwrite status/dismissed_at/resolved_at — preserve lifecycle state
			}),
		}).
		Create(rec).Error
}

func (r *OptimizationRecommendationRepo) GetByID(ctx context.Context, id string) (*model.OptimizationRecommendation, error) {
	var rec model.OptimizationRecommendation
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Preload("VM").
		First(&rec, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("recommendation not found: %w", err)
	}
	return &rec, nil
}

func (r *OptimizationRecommendationRepo) List(ctx context.Context, filter port.RecommendationFilter, page port.Page) (*port.PageResult[model.OptimizationRecommendation], error) {
	var items []model.OptimizationRecommendation
	var total int64

	q := r.db.WithContext(ctx).Model(&model.OptimizationRecommendation{})

	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Severity != "" {
		q = q.Where("severity = ?", filter.Severity)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	} else {
		// Default: show active recommendations
		q = q.Where("status = ?", model.RecommendationStatusActive)
	}
	if filter.HypervisorID != "" {
		q = q.Where("hypervisor_id = ?", filter.HypervisorID)
	}
	if filter.VMID != "" {
		q = q.Where("vm_id = ?", filter.VMID)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.
		Preload("Hypervisor").
		Preload("VM").
		Order("score DESC, severity DESC, created_at DESC").
		Offset(offset).Limit(page.Size).
		Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.OptimizationRecommendation]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *OptimizationRecommendationRepo) UpdateStatus(ctx context.Context, id string, status model.RecommendationStatus, changedBy *string, note string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().UTC(),
	}
	now := time.Now().UTC()
	switch status {
	case model.RecommendationStatusDismissed:
		updates["dismissed_at"] = now
		if changedBy != nil {
			// Store as UUID — parse best-effort
			updates["dismissed_by"] = *changedBy
		}
	case model.RecommendationStatusResolved:
		updates["resolved_at"] = now
	}
	return r.db.WithContext(ctx).
		Model(&model.OptimizationRecommendation{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *OptimizationRecommendationRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&model.OptimizationRecommendation{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64)
	for _, row := range rows {
		result[row.Status] = row.Count
	}
	return result, nil
}

func (r *OptimizationRecommendationRepo) CountBySeverity(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Severity string
		Count    int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&model.OptimizationRecommendation{}).
		Select("severity, COUNT(*) as count").
		Where("status = ?", model.RecommendationStatusActive).
		Group("severity").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64)
	for _, row := range rows {
		result[row.Severity] = row.Count
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RecommendationHistoryRepo
// ─────────────────────────────────────────────────────────────────────────────

type RecommendationHistoryRepo struct{ db *gorm.DB }

func NewRecommendationHistoryRepo(db *gorm.DB) *RecommendationHistoryRepo {
	return &RecommendationHistoryRepo{db: db}
}

func (r *RecommendationHistoryRepo) Create(ctx context.Context, h *model.RecommendationHistory) error {
	return r.db.WithContext(ctx).Create(h).Error
}

func (r *RecommendationHistoryRepo) ListByRecommendation(ctx context.Context, recommendationID string) ([]model.RecommendationHistory, error) {
	var items []model.RecommendationHistory
	err := r.db.WithContext(ctx).
		Where("recommendation_id = ?", recommendationID).
		Order("created_at DESC").
		Find(&items).Error
	return items, err
}
