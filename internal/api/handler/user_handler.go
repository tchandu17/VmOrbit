package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// UserHandler handles user management REST endpoints.
type UserHandler struct {
	svc port.UserService
	log logger.Logger
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(svc port.UserService, log logger.Logger) *UserHandler {
	return &UserHandler{svc: svc, log: log}
}

// List godoc
// @Summary      List users
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /users [get]
func (h *UserHandler) List(c *gin.Context) {
	result, err := h.svc.List(c.Request.Context(), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

type createUserRequest struct {
	Email     string   `json:"email"     binding:"required,email"`
	Username  string   `json:"username"  binding:"required,min=3"`
	Password  string   `json:"password"  binding:"required,min=8"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	RoleIDs   []string `json:"role_ids"`
}

// Create godoc
// @Summary      Create a user
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      createUserRequest  true  "User details"
// @Success      201   {object}  APIResponse
// @Router       /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	user, err := h.svc.Create(c.Request.Context(), port.CreateUserRequest{
		Email:     req.Email,
		Username:  req.Username,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		RoleIDs:   req.RoleIDs,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, user)
}

// GetByID godoc
// @Summary      Get user by ID
// @Tags         users
// @Security     BearerAuth
// @Param        id  path      string  true  "User ID"
// @Success      200  {object}  APIResponse
// @Router       /users/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
	user, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, user)
}

type updateUserRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	IsActive  *bool   `json:"is_active"`
}

// Update godoc
// @Summary      Update user
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string            true  "User ID"
// @Param        body  body      updateUserRequest  true  "Update fields"
// @Success      200   {object}  APIResponse
// @Router       /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	user, err := h.svc.Update(c.Request.Context(), c.Param("id"), port.UpdateUserRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  req.IsActive,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, user)
}

// Delete godoc
// @Summary      Delete user
// @Tags         users
// @Security     BearerAuth
// @Param        id  path  string  true  "User ID"
// @Success      204
// @Router       /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// AssignRole godoc
// @Summary      Assign role to user
// @Tags         users
// @Security     BearerAuth
// @Param        id      path  string  true  "User ID"
// @Param        roleId  path  string  true  "Role ID"
// @Success      204
// @Router       /users/{id}/roles/{roleId} [post]
func (h *UserHandler) AssignRole(c *gin.Context) {
	if err := h.svc.AssignRole(c.Request.Context(), c.Param("id"), c.Param("roleId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// RevokeRole godoc
// @Summary      Revoke role from user
// @Tags         users
// @Security     BearerAuth
// @Param        id      path  string  true  "User ID"
// @Param        roleId  path  string  true  "Role ID"
// @Success      204
// @Router       /users/{id}/roles/{roleId} [delete]
func (h *UserHandler) RevokeRole(c *gin.Context) {
	if err := h.svc.RevokeRole(c.Request.Context(), c.Param("id"), c.Param("roleId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password"     binding:"required,min=8"`
}

// ChangePassword godoc
// @Summary      Change user password
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string                true  "User ID"
// @Param        body  body  changePasswordRequest  true  "Passwords"
// @Success      204
// @Router       /users/{id}/password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), c.Param("id"), req.CurrentPassword, req.NewPassword); err != nil {
		badRequest(c, err.Error())
		return
	}
	noContent(c)
}

// GetPermissions godoc
// @Summary      Get all permissions for a user
// @Tags         users
// @Security     BearerAuth
// @Param        id  path  string  true  "User ID"
// @Success      200  {object}  APIResponse
// @Router       /users/{id}/permissions [get]
func (h *UserHandler) GetPermissions(c *gin.Context) {
	perms, err := h.svc.GetPermissions(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, perms)
}
