package service

import (
	"context"
	"fmt"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// Role service
// ─────────────────────────────────────────────────────────────────────────────

type roleService struct {
	roles port.RoleRepository
	perms port.PermissionRepository
	audit port.AuditService
	log   logger.Logger
}

// NewRoleService creates a new role service.
func NewRoleService(roles port.RoleRepository, perms port.PermissionRepository, audit port.AuditService, log logger.Logger) port.RoleService {
	return &roleService{roles: roles, perms: perms, audit: audit, log: log}
}

func (s *roleService) Create(ctx context.Context, req port.CreateRoleRequest) (*model.Role, error) {
	role := &model.Role{
		Name:        req.Name,
		Description: req.Description,
	}
	if err := s.roles.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}

	for _, permID := range req.PermissionIDs {
		if err := s.roles.AssignPermission(ctx, role.ID.String(), permID); err != nil {
			s.log.Warn("failed to assign permission during role creation",
				logger.String("role_id", role.ID.String()),
				logger.String("perm_id", permID),
				logger.Error(err),
			)
		}
	}

	// Reload with permissions
	role, _ = s.roles.GetByID(ctx, role.ID.String())

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionCreate,
		Resource:   "role",
		ResourceID: role.ID.String(),
		Success:    true,
	})
	return role, nil
}

func (s *roleService) GetByID(ctx context.Context, id string) (*model.Role, error) {
	return s.roles.GetByID(ctx, id)
}

func (s *roleService) Update(ctx context.Context, id string, req port.UpdateRoleRequest) (*model.Role, error) {
	role, err := s.roles.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		role.Name = *req.Name
	}
	if req.Description != nil {
		role.Description = *req.Description
	}
	if err := s.roles.Update(ctx, role); err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionUpdate,
		Resource:   "role",
		ResourceID: id,
		Success:    true,
	})
	return role, nil
}

func (s *roleService) Delete(ctx context.Context, id string) error {
	if err := s.roles.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionDelete,
		Resource:   "role",
		ResourceID: id,
		Success:    true,
	})
	return nil
}

func (s *roleService) List(ctx context.Context) ([]model.Role, error) {
	return s.roles.List(ctx)
}

func (s *roleService) AssignPermission(ctx context.Context, roleID, permissionID string) error {
	return s.roles.AssignPermission(ctx, roleID, permissionID)
}

func (s *roleService) RevokePermission(ctx context.Context, roleID, permissionID string) error {
	return s.roles.RevokePermission(ctx, roleID, permissionID)
}

func (s *roleService) SetPermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	role, err := s.roles.GetByID(ctx, roleID)
	if err != nil {
		return err
	}

	// Fetch all requested permissions
	var newPerms []model.Permission
	for _, pid := range permissionIDs {
		p, err := s.perms.GetByID(ctx, pid)
		if err != nil {
			return fmt.Errorf("permission %s not found: %w", pid, err)
		}
		newPerms = append(newPerms, *p)
	}

	// Build current set
	currentSet := make(map[string]bool)
	for _, p := range role.Permissions {
		currentSet[p.ID.String()] = true
	}

	// Build desired set
	desiredSet := make(map[string]bool)
	for _, pid := range permissionIDs {
		desiredSet[pid] = true
	}

	// Revoke permissions no longer desired
	for _, p := range role.Permissions {
		if !desiredSet[p.ID.String()] {
			if err := s.roles.RevokePermission(ctx, roleID, p.ID.String()); err != nil {
				return err
			}
		}
	}

	// Assign new permissions
	for _, pid := range permissionIDs {
		if !currentSet[pid] {
			if err := s.roles.AssignPermission(ctx, roleID, pid); err != nil {
				return err
			}
		}
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionUpdate,
		Resource:    "role",
		ResourceID:  roleID,
		Description: fmt.Sprintf("set %d permissions", len(newPerms)),
		Success:     true,
	})
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Permission service
// ─────────────────────────────────────────────────────────────────────────────

type permissionService struct {
	repo  port.PermissionRepository
	audit port.AuditService
	log   logger.Logger
}

// NewPermissionService creates a new permission service.
func NewPermissionService(repo port.PermissionRepository, audit port.AuditService, log logger.Logger) port.PermissionService {
	return &permissionService{repo: repo, audit: audit, log: log}
}

func (s *permissionService) Create(ctx context.Context, req port.CreatePermissionRequest) (*model.Permission, error) {
	perm := &model.Permission{
		Resource: req.Resource,
		Action:   req.Action,
	}
	if err := s.repo.Create(ctx, perm); err != nil {
		return nil, fmt.Errorf("creating permission: %w", err)
	}
	return perm, nil
}

func (s *permissionService) GetByID(ctx context.Context, id string) (*model.Permission, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *permissionService) List(ctx context.Context) ([]model.Permission, error) {
	return s.repo.List(ctx)
}

func (s *permissionService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
