package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ProvisioningJobRepo is the GORM-backed provisioning job repository.
type ProvisioningJobRepo struct{ db *gorm.DB }

// NewProvisioningJobRepo creates a new ProvisioningJobRepo.
func NewProvisioningJobRepo(db *gorm.DB) *ProvisioningJobRepo {
	return &ProvisioningJobRepo{db: db}
}

func (r *ProvisioningJobRepo) Create(ctx context.Context, job *model.ProvisioningJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *ProvisioningJobRepo) GetByID(ctx context.Context, id string) (*model.ProvisioningJob, error) {
	var job model.ProvisioningJob
	err := r.db.WithContext(ctx).
		Preload("Template").
		Preload("SourceVM").
		Preload("ResultVM").
		First(&job, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("provisioning job not found: %w", err)
	}
	return &job, nil
}

func (r *ProvisioningJobRepo) Update(ctx context.Context, job *model.ProvisioningJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *ProvisioningJobRepo) List(ctx context.Context, filter port.ProvisioningJobFilter, page port.Page) (*port.PageResult[model.ProvisioningJob], error) {
	var items []model.ProvisioningJob
	var total int64

	q := r.db.WithContext(ctx).Model(&model.ProvisioningJob{})
	if filter.HypervisorID != "" {
		q = q.Where("hypervisor_id = ?", filter.HypervisorID)
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("created_at DESC").
		Offset(offset).Limit(page.Size).
		Preload("Template").
		Preload("SourceVM").
		Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.ProvisioningJob]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}
