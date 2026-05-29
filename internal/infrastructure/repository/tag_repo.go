package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// TagRepo is the GORM-backed Tag repository.
type TagRepo struct{ db *gorm.DB }

// NewTagRepo creates a new TagRepo.
func NewTagRepo(db *gorm.DB) *TagRepo { return &TagRepo{db: db} }

func (r *TagRepo) Create(ctx context.Context, tag *model.Tag) error {
	return r.db.WithContext(ctx).Create(tag).Error
}

func (r *TagRepo) GetByID(ctx context.Context, id string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.WithContext(ctx).First(&tag, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("tag not found: %w", err)
	}
	return &tag, nil
}

func (r *TagRepo) GetByName(ctx context.Context, name string) (*model.Tag, error) {
	var tag model.Tag
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error; err != nil {
		return nil, fmt.Errorf("tag not found: %w", err)
	}
	return &tag, nil
}

func (r *TagRepo) Update(ctx context.Context, tag *model.Tag) error {
	return r.db.WithContext(ctx).Save(tag).Error
}

func (r *TagRepo) Delete(ctx context.Context, id string) error {
	// Remove all VM associations first, then delete the tag.
	if err := r.db.WithContext(ctx).
		Exec("DELETE FROM vm_tags WHERE tag_id = ?", id).Error; err != nil {
		return fmt.Errorf("removing vm_tags: %w", err)
	}
	return r.db.WithContext(ctx).Delete(&model.Tag{}, "id = ?", id).Error
}

func (r *TagRepo) List(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *TagRepo) AddToVM(ctx context.Context, vmID, tagID string) error {
	// Use INSERT ... ON CONFLICT DO NOTHING for idempotency.
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Exec("INSERT INTO vm_tags (vm_id, tag_id) VALUES (?, ?)", vmID, tagID).Error
}

func (r *TagRepo) RemoveFromVM(ctx context.Context, vmID, tagID string) error {
	return r.db.WithContext(ctx).
		Exec("DELETE FROM vm_tags WHERE vm_id = ? AND tag_id = ?", vmID, tagID).Error
}

func (r *TagRepo) ListByVM(ctx context.Context, vmID string) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.WithContext(ctx).
		Joins("JOIN vm_tags ON vm_tags.tag_id = tags.id").
		Where("vm_tags.vm_id = ?", vmID).
		Order("tags.name ASC").
		Find(&tags).Error
	return tags, err
}

func (r *TagRepo) ListVMsByTag(ctx context.Context, tagID string) ([]string, error) {
	var vmIDs []string
	err := r.db.WithContext(ctx).
		Model(&model.VMTag{}).
		Where("tag_id = ?", tagID).
		Pluck("vm_id", &vmIDs).Error
	return vmIDs, err
}
