package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// RoleRepo is the GORM-backed role repository.
type RoleRepo struct{ db *gorm.DB }

// NewRoleRepo creates a new RoleRepo.
func NewRoleRepo(db *gorm.DB) *RoleRepo { return &RoleRepo{db: db} }

func (r *RoleRepo) Create(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *RoleRepo) GetByID(ctx context.Context, id string) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).
		Preload("Permissions").
		First(&role, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("role not found: %w", err)
	}
	return &role, nil
}

func (r *RoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).
		Preload("Permissions").
		Where("name = ?", name).
		First(&role).Error
	if err != nil {
		return nil, fmt.Errorf("role not found: %w", err)
	}
	return &role, nil
}

func (r *RoleRepo) Update(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

func (r *RoleRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.Role{}, "id = ?", id).Error
}

func (r *RoleRepo) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).
		Preload("Permissions").
		Order("name ASC").
		Find(&roles).Error
	return roles, err
}

func (r *RoleRepo) AssignPermission(ctx context.Context, roleID, permissionID string) error {
	role, err := r.GetByID(ctx, roleID)
	if err != nil {
		return err
	}
	var perm model.Permission
	if err := r.db.WithContext(ctx).First(&perm, "id = ?", permissionID).Error; err != nil {
		return fmt.Errorf("permission not found: %w", err)
	}
	return r.db.WithContext(ctx).Model(role).Association("Permissions").Append(&perm)
}

func (r *RoleRepo) RevokePermission(ctx context.Context, roleID, permissionID string) error {
	role, err := r.GetByID(ctx, roleID)
	if err != nil {
		return err
	}
	var perm model.Permission
	if err := r.db.WithContext(ctx).First(&perm, "id = ?", permissionID).Error; err != nil {
		return fmt.Errorf("permission not found: %w", err)
	}
	return r.db.WithContext(ctx).Model(role).Association("Permissions").Delete(&perm)
}

// PermissionRepo is the GORM-backed permission repository.
type PermissionRepo struct{ db *gorm.DB }

// NewPermissionRepo creates a new PermissionRepo.
func NewPermissionRepo(db *gorm.DB) *PermissionRepo { return &PermissionRepo{db: db} }

func (r *PermissionRepo) Create(ctx context.Context, perm *model.Permission) error {
	return r.db.WithContext(ctx).Create(perm).Error
}

func (r *PermissionRepo) GetByID(ctx context.Context, id string) (*model.Permission, error) {
	var perm model.Permission
	if err := r.db.WithContext(ctx).First(&perm, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("permission not found: %w", err)
	}
	return &perm, nil
}

func (r *PermissionRepo) GetByResourceAction(ctx context.Context, resource, action string) (*model.Permission, error) {
	var perm model.Permission
	err := r.db.WithContext(ctx).
		Where("resource = ? AND action = ?", resource, action).
		First(&perm).Error
	if err != nil {
		return nil, fmt.Errorf("permission not found: %w", err)
	}
	return &perm, nil
}

func (r *PermissionRepo) List(ctx context.Context) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).
		Order("resource ASC, action ASC").
		Find(&perms).Error
	return perms, err
}

func (r *PermissionRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.Permission{}, "id = ?", id).Error
}

// Ensure interfaces are satisfied at compile time.
var _ port.RoleRepository = (*RoleRepo)(nil)
var _ port.PermissionRepository = (*PermissionRepo)(nil)
