package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// HypervisorGroupRepo is the GORM-backed hypervisor group repository.
type HypervisorGroupRepo struct{ db *gorm.DB }

// NewHypervisorGroupRepo creates a new HypervisorGroupRepo.
func NewHypervisorGroupRepo(db *gorm.DB) *HypervisorGroupRepo {
	return &HypervisorGroupRepo{db: db}
}

func (r *HypervisorGroupRepo) Create(ctx context.Context, g *model.HypervisorGroup) error {
	return r.db.WithContext(ctx).Create(g).Error
}

func (r *HypervisorGroupRepo) GetByID(ctx context.Context, id string) (*model.HypervisorGroup, error) {
	var g model.HypervisorGroup
	if err := r.db.WithContext(ctx).First(&g, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("hypervisor group not found: %w", err)
	}
	return &g, nil
}

func (r *HypervisorGroupRepo) Update(ctx context.Context, g *model.HypervisorGroup) error {
	return r.db.WithContext(ctx).Save(g).Error
}

func (r *HypervisorGroupRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.HypervisorGroup{}, "id = ?", id).Error
}

func (r *HypervisorGroupRepo) List(ctx context.Context, page port.Page) (*port.PageResult[model.HypervisorGroup], error) {
	var items []model.HypervisorGroup
	var total int64

	q := r.db.WithContext(ctx).Model(&model.HypervisorGroup{})
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

	return &port.PageResult[model.HypervisorGroup]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}
