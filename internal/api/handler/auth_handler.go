package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	svc     port.AuthService
	userSvc port.UserService
	log     logger.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc port.AuthService, userSvc port.UserService, log logger.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, userSvc: userSvc, log: log}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Login godoc
// @Summary      Login
// @Description  Authenticate with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      loginRequest  true  "Credentials"
// @Success      200   {object}  APIResponse
// @Failure      400   {object}  APIResponse
// @Failure      401   {object}  APIResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	tokens, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		unauthorized(c, err.Error())
		return
	}

	ok(c, tokens)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh godoc
// @Summary      Refresh tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      refreshRequest  true  "Refresh token"
// @Success      200   {object}  APIResponse
// @Failure      401   {object}  APIResponse
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	tokens, err := h.svc.RefreshTokens(c.Request.Context(), req.RefreshToken)
	if err != nil {
		unauthorized(c, err.Error())
		return
	}

	ok(c, tokens)
}

// Logout godoc
// @Summary      Logout
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      refreshRequest  true  "Refresh token"
// @Success      204
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	_ = h.svc.Logout(c.Request.Context(), req.RefreshToken)
	noContent(c)
}

// Me godoc
// @Summary      Get current user profile
// @Tags         auth
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(string)
	if uid == "" {
		unauthorized(c, "not authenticated")
		return
	}
	user, err := h.userSvc.GetByID(c.Request.Context(), uid)
	if err != nil {
		notFound(c, "user not found")
		return
	}
	ok(c, user)
}
