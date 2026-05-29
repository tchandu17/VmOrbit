package service

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

type userService struct {
	repo  port.UserRepository
	audit port.AuditService
	log   logger.Logger
}

// NewUserService creates a new user service.
func NewUserService(repo port.UserRepository, audit port.AuditService, log logger.Logger) port.UserService {
	return &userService{repo: repo, audit: audit, log: log}
}

func (s *userService) Create(ctx context.Context, req port.CreateUserRequest) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user := &model.User{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		IsActive:     true,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	// Assign initial roles if provided
	for _, roleID := range req.RoleIDs {
		if err := s.repo.AssignRole(ctx, user.ID.String(), roleID); err != nil {
			s.log.Warn("failed to assign role during user creation",
				logger.String("user_id", user.ID.String()),
				logger.String("role_id", roleID),
				logger.Error(err),
			)
		}
	}

	// Reload with roles
	user, _ = s.repo.GetByID(ctx, user.ID.String())

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionCreate,
		Resource:   "user",
		ResourceID: user.ID.String(),
		Success:    true,
	})

	return user, nil
}

func (s *userService) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *userService) Update(ctx context.Context, id string, req port.UpdateUserRequest) (*model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionUpdate,
		Resource:   "user",
		ResourceID: id,
		Success:    true,
	})

	return user, nil
}

func (s *userService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionDelete,
		Resource:   "user",
		ResourceID: id,
		Success:    true,
	})
	return nil
}

func (s *userService) List(ctx context.Context, page port.Page) (*port.PageResult[model.User], error) {
	return s.repo.List(ctx, page)
}

func (s *userService) AssignRole(ctx context.Context, userID, roleID string) error {
	if err := s.repo.AssignRole(ctx, userID, roleID); err != nil {
		return fmt.Errorf("assigning role: %w", err)
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionUpdate,
		Resource:    "user",
		ResourceID:  userID,
		Description: fmt.Sprintf("assigned role %s", roleID),
		Success:     true,
	})
	return nil
}

func (s *userService) RevokeRole(ctx context.Context, userID, roleID string) error {
	if err := s.repo.RevokeRole(ctx, userID, roleID); err != nil {
		return fmt.Errorf("revoking role: %w", err)
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionUpdate,
		Resource:    "user",
		ResourceID:  userID,
		Description: fmt.Sprintf("revoked role %s", roleID),
		Success:     true,
	})
	return nil
}

func (s *userService) GetPermissions(ctx context.Context, userID string) ([]model.Permission, error) {
	return s.repo.GetPermissions(ctx, userID)
}

func (s *userService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user.PasswordHash = string(hash)
	if err := s.repo.Update(ctx, user); err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionUpdate,
		Resource:    "user",
		ResourceID:  userID,
		Description: "password changed",
		Success:     true,
	})
	return nil
}
