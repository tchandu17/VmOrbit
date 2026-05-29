package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// VMRepo is the GORM-backed VM repository.
type VMRepo struct{ db *gorm.DB }

// NewVMRepo creates a new VMRepo.
func NewVMRepo(db *gorm.DB) *VMRepo { return &VMRepo{db: db} }

func (r *VMRepo) Create(ctx context.Context, vm *model.VM) error {
	return r.db.WithContext(ctx).Create(vm).Error
}

func (r *VMRepo) GetByID(ctx context.Context, id string) (*model.VM, error) {
	var vm model.VM
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Preload("Snapshots", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("TagObjects").
		First(&vm, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("vm not found: %w", err)
	}
	return &vm, nil
}

func (r *VMRepo) GetByIDs(ctx context.Context, ids []string) ([]model.VM, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var vms []model.VM
	if err := r.db.WithContext(ctx).
		Preload("Hypervisor").
		Where("id IN ?", ids).
		Find(&vms).Error; err != nil {
		return nil, err
	}
	return vms, nil
}

func (r *VMRepo) GetByProviderID(ctx context.Context, hypervisorID, providerVMID string) (*model.VM, error) {
	var vm model.VM
	err := r.db.WithContext(ctx).
		Where("hypervisor_id = ? AND provider_vm_id = ?", hypervisorID, providerVMID).
		First(&vm).Error
	if err != nil {
		return nil, fmt.Errorf("vm not found: %w", err)
	}
	return &vm, nil
}

func (r *VMRepo) Update(ctx context.Context, vm *model.VM) error {
	return r.db.WithContext(ctx).Save(vm).Error
}

func (r *VMRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.VM{}, "id = ?", id).Error
}

func (r *VMRepo) List(ctx context.Context, filter port.VMFilter, page port.Page) (*port.PageResult[model.VM], error) {
	var vms []model.VM
	var total int64

	q := r.db.WithContext(ctx).Model(&model.VM{})
	if filter.HypervisorID != "" {
		q = q.Where("hypervisor_id = ?", filter.HypervisorID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	// Tag filtering: only return VMs that have ALL of the requested tags.
	for _, tagID := range filter.TagIDs {
		q = q.Where("EXISTS (SELECT 1 FROM vm_tags WHERE vm_tags.vm_id = vms.id AND vm_tags.tag_id = ?)", tagID)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Preload("TagObjects").Offset(offset).Limit(page.Size).Find(&vms).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.VM]{
		Items:      vms,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *VMRepo) UpdateStatus(ctx context.Context, id string, status model.VMStatus) error {
	return r.db.WithContext(ctx).
		Model(&model.VM{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// BulkUpsert inserts or updates VMs based on (hypervisor_id, provider_vm_id).
// All inventory fields are updated on conflict.
func (r *VMRepo) BulkUpsert(ctx context.Context, vms []model.VM) error {
	if len(vms) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "hypervisor_id"}, {Name: "provider_vm_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "status",
				"cpu_count", "memory_mb", "disk_gb",
				"ip_addresses", "mac_address", "network_name",
				"guest_os", "guest_os_type", "tools_status",
				"tags", "metadata",
				"updated_at",
			}),
		}).
		CreateInBatches(vms, 100).Error
}

// ListProviderIDs returns all provider_vm_id values for a hypervisor (non-deleted).
func (r *VMRepo) ListProviderIDs(ctx context.Context, hypervisorID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&model.VM{}).
		Where("hypervisor_id = ?", hypervisorID).
		Pluck("provider_vm_id", &ids).Error
	return ids, err
}

// MarkDeletedByProviderIDs soft-deletes VMs for a hypervisor whose provider_vm_id
// is NOT in the activeIDs set. Returns the number of rows affected.
func (r *VMRepo) MarkDeletedByProviderIDs(ctx context.Context, hypervisorID string, activeIDs []string) (int64, error) {
	if len(activeIDs) == 0 {
		// Safety guard: never delete everything if the provider returned no VMs.
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Where("hypervisor_id = ? AND provider_vm_id NOT IN ?", hypervisorID, activeIDs).
		Delete(&model.VM{})
	return result.RowsAffected, result.Error
}
