package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// NetworkRepo is the GORM-backed network repository.
type NetworkRepo struct{ db *gorm.DB }

// NewNetworkRepo creates a new NetworkRepo.
func NewNetworkRepo(db *gorm.DB) *NetworkRepo { return &NetworkRepo{db: db} }

// BulkUpsert inserts or updates networks based on (hypervisor_id, provider_id).
func (r *NetworkRepo) BulkUpsert(ctx context.Context, networks []model.Network) error {
	if len(networks) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hypervisor_id"}, {Name: "provider_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "type", "vlan", "accessible", "updated_at",
			}),
		}).
		CreateInBatches(networks, 100).Error
}

// List returns all networks for a hypervisor.
func (r *NetworkRepo) List(ctx context.Context, hypervisorID string) ([]model.Network, error) {
	var networks []model.Network
	err := r.db.WithContext(ctx).
		Where("hypervisor_id = ?", hypervisorID).
		Find(&networks).Error
	return networks, err
}
