package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// UserRepo is the GORM-backed user repository.
type UserRepo struct{ db *gorm.DB }

// NewUserRepo creates a new UserRepo.
func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		First(&user, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		Where("email = ?", email).
		First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (r *UserRepo) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, "id = ?", id).Error
}

func (r *UserRepo) List(ctx context.Context, page port.Page) (*port.PageResult[model.User], error) {
	var users []model.User
	var total int64

	q := r.db.WithContext(ctx).Model(&model.User{})
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Preload("Roles").Offset(offset).Limit(page.Size).Find(&users).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.User]{
		Items:      users,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *UserRepo) AssignRole(ctx context.Context, userID, roleID string) error {
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	var role model.Role
	if err := r.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		return fmt.Errorf("role not found: %w", err)
	}
	return r.db.WithContext(ctx).Model(user).Association("Roles").Append(&role)
}

func (r *UserRepo) RevokeRole(ctx context.Context, userID, roleID string) error {
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	var role model.Role
	if err := r.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		return fmt.Errorf("role not found: %w", err)
	}
	return r.db.WithContext(ctx).Model(user).Association("Roles").Delete(&role)
}

func (r *UserRepo) GetPermissions(ctx context.Context, userID string) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.WithContext(ctx).
		Distinct("permissions.*").
		Table("permissions").
		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Joins("JOIN user_roles ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ?", userID).
		Find(&perms).Error
	return perms, err
}

// Ensure interface is satisfied at compile time.
var _ port.UserRepository = (*UserRepo)(nil)
