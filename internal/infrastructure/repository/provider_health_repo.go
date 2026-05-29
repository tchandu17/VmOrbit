package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ProviderHealthRepo is the GORM-backed provider health repository.
type ProviderHealthRepo struct{ db *gorm.DB }

// NewProviderHealthRepo creates a new ProviderHealthRepo.
func NewProviderHealthRepo(db *gorm.DB) *ProviderHealthRepo {
	return &ProviderHealthRepo{db: db}
}

// Upsert inserts or updates the health snapshot for a hypervisor.
// Uses PostgreSQL ON CONFLICT DO UPDATE so it is safe to call on every check cycle.
func (r *ProviderHealthRepo) Upsert(ctx context.Context, h *model.ProviderHealth) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hypervisor_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"updated_at",
				"status",
				"online",
				"last_seen_at",
				"last_check_at",
				"consecutive_fails",
				"latency_ms",
				"avg_latency_ms",
				"peak_latency_ms",
				"last_sync_at",
				"last_sync_status",
				"sync_failures24h",
				"vm_count",
				"tasks_total24h",
				"tasks_failed24h",
				"task_failure_rate",
				"auth_failures24h",
				"inventory_age_minutes",
				"inventory_stale",
				"health_score",
			}),
		}).
		Create(h).Error
}

// GetByHypervisorID returns the current health snapshot for a hypervisor.
func (r *ProviderHealthRepo) GetByHypervisorID(ctx context.Context, hypervisorID string) (*model.ProviderHealth, error) {
	var h model.ProviderHealth
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Where("hypervisor_id = ?", hypervisorID).
		First(&h).Error; err != nil {
		return nil, fmt.Errorf("provider health not found: %w", err)
	}
	return &h, nil
}

// List returns health snapshots for all hypervisors, joined with hypervisor data.
func (r *ProviderHealthRepo) List(ctx context.Context) ([]model.ProviderHealth, error) {
	var items []model.ProviderHealth
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Order("health_score ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// AppendHistory inserts a history row.
func (r *ProviderHealthRepo) AppendHistory(ctx context.Context, h *model.ProviderHealthHistory) error {
	return r.db.WithContext(ctx).Create(h).Error
}

// GetHistory returns the last N history rows for a hypervisor, newest-first.
func (r *ProviderHealthRepo) GetHistory(ctx context.Context, hypervisorID string, limit int) ([]model.ProviderHealthHistory, error) {
	if limit <= 0 {
		limit = 60
	}
	var items []model.ProviderHealthHistory
	if err := r.db.WithContext(ctx).
		Where("hypervisor_id = ?", hypervisorID).
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
