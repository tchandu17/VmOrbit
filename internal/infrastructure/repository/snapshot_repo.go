package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// SnapshotRepo is the GORM-backed snapshot repository.
type SnapshotRepo struct{ db *gorm.DB }

// NewSnapshotRepo creates a new SnapshotRepo.
func NewSnapshotRepo(db *gorm.DB) *SnapshotRepo { return &SnapshotRepo{db: db} }

func (r *SnapshotRepo) Create(ctx context.Context, s *model.Snapshot) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *SnapshotRepo) GetByID(ctx context.Context, id string) (*model.Snapshot, error) {
	var s model.Snapshot
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("snapshot not found: %w", err)
	}
	return &s, nil
}

func (r *SnapshotRepo) GetByProviderID(ctx context.Context, vmID, providerID string) (*model.Snapshot, error) {
	var s model.Snapshot
	if err := r.db.WithContext(ctx).
		Where("vm_id = ? AND provider_id = ?", vmID, providerID).
		First(&s).Error; err != nil {
		return nil, fmt.Errorf("snapshot not found: %w", err)
	}
	return &s, nil
}

func (r *SnapshotRepo) ListByVMID(ctx context.Context, vmID string) ([]model.Snapshot, error) {
	var snaps []model.Snapshot
	if err := r.db.WithContext(ctx).
		Where("vm_id = ?", vmID).
		Order("created_at ASC").
		Find(&snaps).Error; err != nil {
		return nil, err
	}
	return snaps, nil
}

func (r *SnapshotRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.Snapshot{}, "id = ?", id).Error
}

func (r *SnapshotRepo) DeleteByProviderID(ctx context.Context, vmID, providerID string) error {
	return r.db.WithContext(ctx).
		Where("vm_id = ? AND provider_id = ?", vmID, providerID).
		Delete(&model.Snapshot{}).Error
}

// SetCurrentSnapshot marks the given snapshot as current and clears the flag
// on all other snapshots for the same VM. Runs in a single transaction.
func (r *SnapshotRepo) SetCurrentSnapshot(ctx context.Context, vmID, snapshotID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear all current flags for this VM.
		if err := tx.Model(&model.Snapshot{}).
			Where("vm_id = ?", vmID).
			Update("is_current", false).Error; err != nil {
			return err
		}
		// Set the new current snapshot.
		return tx.Model(&model.Snapshot{}).
			Where("id = ?", snapshotID).
			Update("is_current", true).Error
	})
}

// BulkUpsert inserts or updates snapshots based on (vm_id, provider_id).
func (r *SnapshotRepo) BulkUpsert(ctx context.Context, snaps []model.Snapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "vm_id"}, {Name: "provider_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "description", "is_current", "parent_id", "updated_at",
			}),
		}).
		CreateInBatches(snaps, 100).Error
}

// Ensure SnapshotRepo satisfies port.SnapshotRepository.
var _ port.SnapshotRepository = (*SnapshotRepo)(nil)
