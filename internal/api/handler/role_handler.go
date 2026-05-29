package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// RoleHandler handles role and permission management REST endpoints.
type RoleHandler struct {
	roles port.RoleService
	perms port.PermissionService
	log   logger.Logger
}

// NewRoleHandler creates a new RoleHandler.
func NewRoleHandler(roles port.RoleService, perms port.PermissionService, log logger.Logger) *RoleHandler {
	return &RoleHandler{roles: roles, perms: perms, log: log}
}

// ─────────────────────────────────────────────────────────────────────────────
// Roles
// ─────────────────────────────────────────────────────────────────────────────

// ListRoles godoc
// @Summary      List all roles
// @Tags         roles
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /roles [get]
func (h *RoleHandler) ListRoles(c *gin.Context) {
	roles, err := h.roles.List(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, roles)
}

type createRoleRequest struct {
	Name          string   `json:"name"           binding:"required,min=2,max=64"`
	Description   string   `json:"description"`
	PermissionIDs []string `json:"permission_ids"`
}

// CreateRole godoc
// @Summary      Create a role
// @Tags         roles
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      createRoleRequest  true  "Role details"
// @Success      201   {object}  APIResponse
// @Router       /roles [post]
func (h *RoleHandler) CreateRole(c *gin.Context) {
	var req createRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	role, err := h.roles.Create(c.Request.Context(), port.CreateRoleRequest{
		Name:          req.Name,
		Description:   req.Description,
		PermissionIDs: req.PermissionIDs,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, role)
}

// GetRole godoc
// @Summary      Get role by ID
// @Tags         roles
// @Security     BearerAuth
// @Param        id  path  string  true  "Role ID"
// @Success      200  {object}  APIResponse
// @Router       /roles/{id} [get]
func (h *RoleHandler) GetRole(c *gin.Context) {
	role, err := h.roles.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, role)
}

type updateRoleRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// UpdateRole godoc
// @Summary      Update a role
// @Tags         roles
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string            true  "Role ID"
// @Param        body  body  updateRoleRequest  true  "Update fields"
// @Success      200   {object}  APIResponse
// @Router       /roles/{id} [put]
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	role, err := h.roles.Update(c.Request.Context(), c.Param("id"), port.UpdateRoleRequest{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, role)
}

// DeleteRole godoc
// @Summary      Delete a role
// @Tags         roles
// @Security     BearerAuth
// @Param        id  path  string  true  "Role ID"
// @Success      204
// @Router       /roles/{id} [delete]
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	if err := h.roles.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// AssignPermission godoc
// @Summary      Assign permission to role
// @Tags         roles
// @Security     BearerAuth
// @Param        id           path  string  true  "Role ID"
// @Param        permissionId path  string  true  "Permission ID"
// @Success      204
// @Router       /roles/{id}/permissions/{permissionId} [post]
func (h *RoleHandler) AssignPermission(c *gin.Context) {
	if err := h.roles.AssignPermission(c.Request.Context(), c.Param("id"), c.Param("permissionId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// RevokePermission godoc
// @Summary      Revoke permission from role
// @Tags         roles
// @Security     BearerAuth
// @Param        id           path  string  true  "Role ID"
// @Param        permissionId path  string  true  "Permission ID"
// @Success      204
// @Router       /roles/{id}/permissions/{permissionId} [delete]
func (h *RoleHandler) RevokePermission(c *gin.Context) {
	if err := h.roles.RevokePermission(c.Request.Context(), c.Param("id"), c.Param("permissionId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

type setPermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids" binding:"required"`
}

// SetPermissions godoc
// @Summary      Replace all permissions on a role
// @Tags         roles
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string                true  "Role ID"
// @Param        body  body  setPermissionsRequest  true  "Permission IDs"
// @Success      204
// @Router       /roles/{id}/permissions [put]
func (h *RoleHandler) SetPermissions(c *gin.Context) {
	var req setPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := h.roles.SetPermissions(c.Request.Context(), c.Param("id"), req.PermissionIDs); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// ─────────────────────────────────────────────────────────────────────────────
// Permissions
// ─────────────────────────────────────────────────────────────────────────────

// ListPermissions godoc
// @Summary      List all permissions
// @Tags         permissions
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /permissions [get]
func (h *RoleHandler) ListPermissions(c *gin.Context) {
	perms, err := h.perms.List(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, perms)
}

type createPermissionRequest struct {
	Resource string `json:"resource" binding:"required,min=2,max=64"`
	Action   string `json:"action"   binding:"required,min=2,max=64"`
}

// CreatePermission godoc
// @Summary      Create a permission
// @Tags         permissions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  createPermissionRequest  true  "Permission details"
// @Success      201   {object}  APIResponse
// @Router       /permissions [post]
func (h *RoleHandler) CreatePermission(c *gin.Context) {
	var req createPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	perm, err := h.perms.Create(c.Request.Context(), port.CreatePermissionRequest{
		Resource: req.Resource,
		Action:   req.Action,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, perm)
}

// DeletePermission godoc
// @Summary      Delete a permission
// @Tags         permissions
// @Security     BearerAuth
// @Param        id  path  string  true  "Permission ID"
// @Success      204
// @Router       /permissions/{id} [delete]
func (h *RoleHandler) DeletePermission(c *gin.Context) {
	if err := h.perms.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}
