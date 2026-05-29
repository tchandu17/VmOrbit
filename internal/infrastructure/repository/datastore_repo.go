package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// DataStoreRepo is the GORM-backed datastore repository.
type DataStoreRepo struct{ db *gorm.DB }

// NewDataStoreRepo creates a new DataStoreRepo.
func NewDataStoreRepo(db *gorm.DB) *DataStoreRepo { return &DataStoreRepo{db: db} }

// BulkUpsert inserts or updates datastores based on (hypervisor_id, provider_id).
func (r *DataStoreRepo) BulkUpsert(ctx context.Context, stores []model.DataStore) error {
	if len(stores) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hypervisor_id"}, {Name: "provider_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "type", "capacity_gb", "used_gb", "free_gb", "accessible", "updated_at",
			}),
		}).
		CreateInBatches(stores, 100).Error
}

// List returns all datastores for a hypervisor.
func (r *DataStoreRepo) List(ctx context.Context, hypervisorID string) ([]model.DataStore, error) {
	var stores []model.DataStore
	err := r.db.WithContext(ctx).
		Where("hypervisor_id = ?", hypervisorID).
		Find(&stores).Error
	return stores, err
}
