package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// InfrastructureHandler handles infrastructure hierarchy REST endpoints.
type InfrastructureHandler struct {
	svc port.InfrastructureService
	log logger.Logger
}

// NewInfrastructureHandler creates a new InfrastructureHandler.
func NewInfrastructureHandler(svc port.InfrastructureService, log logger.Logger) *InfrastructureHandler {
	return &InfrastructureHandler{svc: svc, log: log}
}

// GetTree godoc
// @Summary      Get infrastructure hierarchy tree
// @Tags         infrastructure
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Success      200  {object}  APIResponse
// @Router       /infrastructure/tree [get]
func (h *InfrastructureHandler) GetTree(c *gin.Context) {
	hypervisorID := c.Query("hypervisor_id")
	tree, err := h.svc.GetTree(c.Request.Context(), hypervisorID)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, tree)
}

// ListHosts godoc
// @Summary      List all hosts/nodes
// @Tags         infrastructure
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Success      200  {object}  APIResponse
// @Router       /hosts [get]
func (h *InfrastructureHandler) ListHosts(c *gin.Context) {
	hypervisorID := c.Query("hypervisor_id")
	hosts, err := h.svc.ListHosts(c.Request.Context(), hypervisorID)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, hosts)
}

// GetHost godoc
// @Summary      Get host details with hosted VMs and datastores
// @Tags         infrastructure
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Host ID"
// @Success      200  {object}  APIResponse
// @Router       /hosts/{id} [get]
func (h *InfrastructureHandler) GetHost(c *gin.Context) {
	detail, err := h.svc.GetHost(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, detail)
}

// ListClusters godoc
// @Summary      List all clusters
// @Tags         infrastructure
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Success      200  {object}  APIResponse
// @Router       /clusters [get]
func (h *InfrastructureHandler) ListClusters(c *gin.Context) {
	hypervisorID := c.Query("hypervisor_id")
	clusters, err := h.svc.ListClusters(c.Request.Context(), hypervisorID)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, clusters)
}

// GetCluster godoc
// @Summary      Get cluster details with hosts
// @Tags         infrastructure
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Cluster ID"
// @Success      200  {object}  APIResponse
// @Router       /clusters/{id} [get]
func (h *InfrastructureHandler) GetCluster(c *gin.Context) {
	cluster, err := h.svc.GetCluster(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, cluster)
}

// ListDataStores godoc
// @Summary      List all datastores
// @Tags         infrastructure
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Success      200  {object}  APIResponse
// @Router       /datastores [get]
func (h *InfrastructureHandler) ListDataStores(c *gin.Context) {
	hypervisorID := c.Query("hypervisor_id")
	stores, err := h.svc.ListDataStores(c.Request.Context(), hypervisorID)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, stores)
}

// ListNetworks godoc
// @Summary      List all virtual networks
// @Tags         infrastructure
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Success      200  {object}  APIResponse
// @Router       /networks [get]
func (h *InfrastructureHandler) ListNetworks(c *gin.Context) {
	hypervisorID := c.Query("hypervisor_id")
	networks, err := h.svc.ListNetworks(c.Request.Context(), hypervisorID)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, networks)
}
