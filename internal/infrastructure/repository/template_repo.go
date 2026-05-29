package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// TemplateRepo is the GORM-backed VM template repository.
type TemplateRepo struct{ db *gorm.DB }

// NewTemplateRepo creates a new TemplateRepo.
func NewTemplateRepo(db *gorm.DB) *TemplateRepo { return &TemplateRepo{db: db} }

func (r *TemplateRepo) Create(ctx context.Context, t *model.VMTemplate) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *TemplateRepo) GetByID(ctx context.Context, id string) (*model.VMTemplate, error) {
	var t model.VMTemplate
	if err := r.db.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	return &t, nil
}

func (r *TemplateRepo) List(ctx context.Context, hypervisorID string, page port.Page) (*port.PageResult[model.VMTemplate], error) {
	var items []model.VMTemplate
	var total int64

	q := r.db.WithContext(ctx).Model(&model.VMTemplate{})
	if hypervisorID != "" {
		q = q.Where("hypervisor_id = ?", hypervisorID)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("name ASC").Offset(offset).Limit(page.Size).Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.VMTemplate]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

// BulkUpsert inserts or updates templates based on (hypervisor_id, provider_id).
func (r *TemplateRepo) BulkUpsert(ctx context.Context, templates []model.VMTemplate) error {
	if len(templates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "hypervisor_id"},
				{Name: "provider_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "description", "guest_os",
				"cpu_count", "memory_mb", "disk_gb",
				"tags", "metadata", "updated_at",
			}),
		}).
		CreateInBatches(templates, 100).Error
}

func (r *TemplateRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.VMTemplate{}, "id = ?", id).Error
}
