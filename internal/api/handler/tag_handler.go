package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// TagHandler handles tag REST endpoints.
type TagHandler struct {
	svc port.TagService
	log logger.Logger
}

// NewTagHandler creates a new TagHandler.
func NewTagHandler(svc port.TagService, log logger.Logger) *TagHandler {
	return &TagHandler{svc: svc, log: log}
}

// List godoc
// @Summary      List all tags
// @Tags         tags
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /tags [get]
func (h *TagHandler) List(c *gin.Context) {
	tags, err := h.svc.List(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, tags)
}

// GetByID godoc
// @Summary      Get tag by ID
// @Tags         tags
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Tag ID"
// @Success      200  {object}  APIResponse
// @Router       /tags/{id} [get]
func (h *TagHandler) GetByID(c *gin.Context) {
	tag, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, tag)
}

type createTagRequest struct {
	Name        string `json:"name"        binding:"required"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// Create godoc
// @Summary      Create a tag
// @Tags         tags
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  createTagRequest  true  "Tag data"
// @Success      201  {object}  APIResponse
// @Router       /tags [post]
func (h *TagHandler) Create(c *gin.Context) {
	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	tag, err := h.svc.Create(c.Request.Context(), port.CreateTagRequest{
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, tag)
}

type updateTagRequest struct {
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
}

// Update godoc
// @Summary      Update a tag
// @Tags         tags
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string           true  "Tag ID"
// @Param        body  body  updateTagRequest  true  "Tag data"
// @Success      200  {object}  APIResponse
// @Router       /tags/{id} [put]
func (h *TagHandler) Update(c *gin.Context) {
	var req updateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	tag, err := h.svc.Update(c.Request.Context(), c.Param("id"), port.UpdateTagRequest{
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, tag)
}

// Delete godoc
// @Summary      Delete a tag
// @Tags         tags
// @Security     BearerAuth
// @Param        id  path  string  true  "Tag ID"
// @Success      204
// @Router       /tags/{id} [delete]
func (h *TagHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// ListByVM godoc
// @Summary      List tags for a VM
// @Tags         tags
// @Security     BearerAuth
// @Param        id  path  string  true  "VM ID"
// @Success      200  {object}  APIResponse
// @Router       /vms/{id}/tags [get]
func (h *TagHandler) ListByVM(c *gin.Context) {
	tags, err := h.svc.ListByVM(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, tags)
}

type addTagToVMRequest struct {
	TagID string `json:"tag_id" binding:"required"`
}

// AddToVM godoc
// @Summary      Add a tag to a VM
// @Tags         tags
// @Security     BearerAuth
// @Accept       json
// @Param        id    path  string           true  "VM ID"
// @Param        body  body  addTagToVMRequest  true  "Tag ID"
// @Success      204
// @Router       /vms/{id}/tags [post]
func (h *TagHandler) AddToVM(c *gin.Context) {
	var req addTagToVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := h.svc.AddToVM(c.Request.Context(), c.Param("id"), req.TagID); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// RemoveFromVM godoc
// @Summary      Remove a tag from a VM
// @Tags         tags
// @Security     BearerAuth
// @Param        id     path  string  true  "VM ID"
// @Param        tagId  path  string  true  "Tag ID"
// @Success      204
// @Router       /vms/{id}/tags/{tagId} [delete]
func (h *TagHandler) RemoveFromVM(c *gin.Context) {
	if err := h.svc.RemoveFromVM(c.Request.Context(), c.Param("id"), c.Param("tagId")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}
