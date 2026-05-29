package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// EnqueueFunc is a function that pushes a task ID onto the Redis work queue.
// It is injected into HypervisorHandler so that SyncInventory can immediately
// enqueue the task rather than waiting for the DB fallback poller.
type EnqueueFunc func(ctx context.Context, taskID string, priority int) error

// HypervisorHandler handles hypervisor REST endpoints.
type HypervisorHandler struct {
	svc     port.HypervisorService
	enqueue EnqueueFunc
	log     logger.Logger
}

// NewHypervisorHandler creates a new HypervisorHandler.
// enqueue may be nil — if so, the task engine's DB fallback poller will pick
// up the task within poll_interval (typically 2 s).
func NewHypervisorHandler(svc port.HypervisorService, enqueue EnqueueFunc, log logger.Logger) *HypervisorHandler {
	return &HypervisorHandler{svc: svc, enqueue: enqueue, log: log}
}

// List godoc
// @Summary      List hypervisors
// @Tags         hypervisors
// @Security     BearerAuth
// @Produce      json
// @Param        page       query     int  false  "Page number"
// @Param        page_size  query     int  false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /hypervisors [get]
func (h *HypervisorHandler) List(c *gin.Context) {
	result, err := h.svc.List(c.Request.Context(), parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// registerHypervisorRequest is the JSON body for POST /hypervisors.
// Provider-specific fields are stored in the Metadata JSONB column.
type registerHypervisorRequest struct {
	Name        string             `json:"name"        binding:"required"`
	Description string             `json:"description"`
	Provider    model.ProviderType `json:"provider"    binding:"required"`
	Host        string             `json:"host"        binding:"required"`
	Port        int                `json:"port"`
	Username    string             `json:"username"`
	Password    string             `json:"password"`
	Token       string             `json:"token"`
	TLSVerify   bool               `json:"tls_verify"`
	Tags        []string           `json:"tags"`

	// VMware-specific
	VCenterURL string `json:"vcenter_url"`
	Datacenter string `json:"datacenter"`

	// Proxmox-specific
	Node           string `json:"node"`
	APITokenID     string `json:"api_token_id"`
	APITokenSecret string `json:"api_token_secret"`
}

// Register godoc
// @Summary      Register a hypervisor
// @Tags         hypervisors
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      registerHypervisorRequest  true  "Hypervisor details"
// @Success      201   {object}  APIResponse
// @Router       /hypervisors [post]
func (h *HypervisorHandler) Register(c *gin.Context) {
	var req registerHypervisorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	// Validate provider value
	switch req.Provider {
	case model.ProviderVMware, model.ProviderESXi, model.ProviderProxmox, model.ProviderKVM, model.ProviderHyperV:
	default:
		badRequest(c, "unsupported provider: "+string(req.Provider))
		return
	}

	// Set sensible default ports
	if req.Port == 0 {
		switch req.Provider {
		case model.ProviderVMware, model.ProviderESXi:
			req.Port = 443
		case model.ProviderProxmox:
			req.Port = 8006
		default:
			req.Port = 443
		}
	}

	// Build provider-specific metadata
	meta := buildMetadata(req)

	// For Proxmox, the API token is the credential
	password := req.Password
	token := req.Token
	if req.Provider == model.ProviderProxmox && req.APITokenSecret != "" {
		token = req.APITokenID
		password = req.APITokenSecret
	}

	hypervisor, err := h.svc.Register(c.Request.Context(), port.RegisterHypervisorRequest{
		Name:        req.Name,
		Description: req.Description,
		Provider:    req.Provider,
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		Password:    password,
		Token:       token,
		TLSVerify:   req.TLSVerify,
		Tags:        req.Tags,
		Metadata:    meta,
	})
	if err != nil {
		if strings.Contains(err.Error(), "unsupported provider") {
			badRequest(c, err.Error())
			return
		}
		internalError(c, err.Error())
		return
	}
	created(c, hypervisor)
}

// GetByID godoc
// @Summary      Get hypervisor by ID
// @Tags         hypervisors
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Hypervisor ID"
// @Success      200  {object}  APIResponse
// @Router       /hypervisors/{id} [get]
func (h *HypervisorHandler) GetByID(c *gin.Context) {
	hypervisor, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "hypervisor not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, hypervisor)
}

// updateHypervisorRequest is the JSON body for PUT /hypervisors/:id.
type updateHypervisorRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Host        *string  `json:"host"`
	Port        *int     `json:"port"`
	Username    *string  `json:"username"`
	Password    *string  `json:"password"`
	Token       *string  `json:"token"`
	TLSVerify   *bool    `json:"tls_verify"`
	Tags        []string `json:"tags"`

	// VMware-specific
	VCenterURL *string `json:"vcenter_url"`
	Datacenter *string `json:"datacenter"`

	// Proxmox-specific
	Node           *string `json:"node"`
	APITokenID     *string `json:"api_token_id"`
	APITokenSecret *string `json:"api_token_secret"`
}

// Update godoc
// @Summary      Update hypervisor
// @Tags         hypervisors
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                   true  "Hypervisor ID"
// @Param        body  body      updateHypervisorRequest  true  "Update fields"
// @Success      200   {object}  APIResponse
// @Router       /hypervisors/{id} [put]
func (h *HypervisorHandler) Update(c *gin.Context) {
	var req updateHypervisorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	// Build metadata from update fields
	meta := buildUpdateMetadata(req)

	hypervisor, err := h.svc.Update(c.Request.Context(), c.Param("id"), port.UpdateHypervisorRequest{
		Name:        req.Name,
		Description: req.Description,
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		Password:    req.Password,
		Token:       req.Token,
		TLSVerify:   req.TLSVerify,
		Tags:        req.Tags,
		Metadata:    meta,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "hypervisor not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	ok(c, hypervisor)
}

// Delete godoc
// @Summary      Delete hypervisor
// @Tags         hypervisors
// @Security     BearerAuth
// @Param        id  path  string  true  "Hypervisor ID"
// @Success      204
// @Router       /hypervisors/{id} [delete]
func (h *HypervisorHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.log.Error("hypervisor delete failed",
			logger.String("id", c.Param("id")),
			logger.Error(err),
		)
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "hypervisor not found")
			return
		}
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// TestConnection godoc
// @Summary      Test hypervisor connectivity
// @Tags         hypervisors
// @Security     BearerAuth
// @Param        id  path  string  true  "Hypervisor ID"
// @Success      200  {object}  APIResponse
// @Router       /hypervisors/{id}/test-connection [post]
func (h *HypervisorHandler) TestConnection(c *gin.Context) {
	if err := h.svc.TestConnection(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "hypervisor not found")
			return
		}
		ok(c, gin.H{"connected": false, "error": err.Error()})
		return
	}
	ok(c, gin.H{"connected": true})
}

// SyncInventory godoc
// @Summary      Trigger inventory sync for a hypervisor
// @Tags         hypervisors
// @Security     BearerAuth
// @Param        id  path  string  true  "Hypervisor ID"
// @Success      202  {object}  APIResponse
// @Router       /hypervisors/{id}/sync [post]
func (h *HypervisorHandler) SyncInventory(c *gin.Context) {
	taskID, err := h.svc.SyncInventory(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			notFound(c, "hypervisor not found")
			return
		}
		internalError(c, err.Error())
		return
	}

	// Immediately push the task onto the Redis queue so workers pick it up
	// without waiting for the DB fallback poller interval.
	if h.enqueue != nil {
		if err := h.enqueue(c.Request.Context(), taskID, 5); err != nil {
			// Non-fatal: the DB poller will recover it within poll_interval.
			h.log.Warn("failed to enqueue sync task into Redis (will be picked up by poller)",
				logger.String("task_id", taskID),
				logger.Error(err),
			)
		}
	}

	c.JSON(202, APIResponse{Success: true, Data: gin.H{"task_id": taskID}})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildMetadata(req registerHypervisorRequest) model.JSONMap {
	meta := model.JSONMap{}
	switch req.Provider {
	case model.ProviderVMware:
		if req.VCenterURL != "" {
			meta["vcenter_url"] = req.VCenterURL
		}
		if req.Datacenter != "" {
			meta["datacenter"] = req.Datacenter
		}
	case model.ProviderProxmox:
		if req.Node != "" {
			meta["node"] = req.Node
		}
		if req.APITokenID != "" {
			meta["api_token_id"] = req.APITokenID
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func buildUpdateMetadata(req updateHypervisorRequest) model.JSONMap {
	meta := model.JSONMap{}
	if req.VCenterURL != nil {
		meta["vcenter_url"] = *req.VCenterURL
	}
	if req.Datacenter != nil {
		meta["datacenter"] = *req.Datacenter
	}
	if req.Node != nil {
		meta["node"] = *req.Node
	}
	if req.APITokenID != nil {
		meta["api_token_id"] = *req.APITokenID
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}
