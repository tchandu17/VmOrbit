package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// ClusterRepo
// ─────────────────────────────────────────────────────────────────────────────

// ClusterRepo is the GORM-backed cluster repository.
type ClusterRepo struct{ db *gorm.DB }

// NewClusterRepo creates a new ClusterRepo.
func NewClusterRepo(db *gorm.DB) *ClusterRepo { return &ClusterRepo{db: db} }

func (r *ClusterRepo) BulkUpsert(ctx context.Context, clusters []model.Cluster) error {
	if len(clusters) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hypervisor_id"}, {Name: "provider_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "total_cpu", "total_memory_mb", "host_count", "vm_count", "metadata", "updated_at",
			}),
		}).
		CreateInBatches(clusters, 100).Error
}

func (r *ClusterRepo) List(ctx context.Context, hypervisorID string) ([]model.Cluster, error) {
	var clusters []model.Cluster
	q := r.db.WithContext(ctx).Model(&model.Cluster{})
	if hypervisorID != "" {
		q = q.Where("hypervisor_id = ?", hypervisorID)
	}
	if err := q.Order("name ASC").Find(&clusters).Error; err != nil {
		return nil, err
	}
	return clusters, nil
}

func (r *ClusterRepo) GetByID(ctx context.Context, id string) (*model.Cluster, error) {
	var c model.Cluster
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Preload("Hosts").
		First(&c, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}
	return &c, nil
}

func (r *ClusterRepo) GetByProviderID(ctx context.Context, hypervisorID, providerID string) (*model.Cluster, error) {
	var c model.Cluster
	if err := r.db.WithContext(ctx).
		Where("hypervisor_id = ? AND provider_id = ?", hypervisorID, providerID).
		First(&c).Error; err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}
	return &c, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// HostRepo
// ─────────────────────────────────────────────────────────────────────────────

// HostRepo is the GORM-backed host repository.
type HostRepo struct{ db *gorm.DB }

// NewHostRepo creates a new HostRepo.
func NewHostRepo(db *gorm.DB) *HostRepo { return &HostRepo{db: db} }

func (r *HostRepo) BulkUpsert(ctx context.Context, hosts []model.Host) error {
	if len(hosts) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hypervisor_id"}, {Name: "provider_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"cluster_id", "name", "status",
				"cpu_model", "cpu_sockets", "cpu_cores", "cpu_threads", "cpu_usage_m_hz",
				"total_memory_mb", "used_memory_mb",
				"hypervisor_version", "uptime_seconds", "vm_count", "metadata", "updated_at",
			}),
		}).
		CreateInBatches(hosts, 100).Error
}

func (r *HostRepo) List(ctx context.Context, hypervisorID string) ([]model.Host, error) {
	var hosts []model.Host
	q := r.db.WithContext(ctx).Model(&model.Host{})
	if hypervisorID != "" {
		q = q.Where("hypervisor_id = ?", hypervisorID)
	}
	if err := q.Preload("Cluster").Order("name ASC").Find(&hosts).Error; err != nil {
		return nil, err
	}
	return hosts, nil
}

func (r *HostRepo) ListByCluster(ctx context.Context, clusterID string) ([]model.Host, error) {
	var hosts []model.Host
	if err := r.db.WithContext(ctx).
		Where("cluster_id = ?", clusterID).
		Order("name ASC").
		Find(&hosts).Error; err != nil {
		return nil, err
	}
	return hosts, nil
}

func (r *HostRepo) GetByID(ctx context.Context, id string) (*model.Host, error) {
	var h model.Host
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Preload("Cluster").
		First(&h, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}
	return &h, nil
}

func (r *HostRepo) GetByProviderID(ctx context.Context, hypervisorID, providerID string) (*model.Host, error) {
	var h model.Host
	if err := r.db.WithContext(ctx).
		Where("hypervisor_id = ? AND provider_id = ?", hypervisorID, providerID).
		First(&h).Error; err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}
	return &h, nil
}

// UpdateVMCount recalculates vm_count for all hosts of a hypervisor by counting
// VMs whose metadata->>'esxi_host' or metadata->>'node' matches the host name.
func (r *HostRepo) UpdateVMCount(ctx context.Context, hypervisorID string) error {
	// Update VMware hosts (esxi_host metadata key)
	if err := r.db.WithContext(ctx).Exec(`
		UPDATE hosts h
		SET vm_count = (
			SELECT COUNT(*) FROM vms v
			WHERE v.hypervisor_id = h.hypervisor_id
			  AND v.metadata->>'esxi_host' = h.name
			  AND v.deleted_at IS NULL
		)
		WHERE h.hypervisor_id = ?
		  AND h.deleted_at IS NULL
	`, hypervisorID).Error; err != nil {
		return err
	}
	// Update Proxmox hosts (node metadata key)
	return r.db.WithContext(ctx).Exec(`
		UPDATE hosts h
		SET vm_count = (
			SELECT COUNT(*) FROM vms v
			WHERE v.hypervisor_id = h.hypervisor_id
			  AND v.metadata->>'node' = h.name
			  AND v.deleted_at IS NULL
		)
		WHERE h.hypervisor_id = ?
		  AND h.deleted_at IS NULL
		  AND h.vm_count = 0
	`, hypervisorID).Error
}
