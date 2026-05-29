package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/api/middleware"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// AnalyticsHandler handles analytics REST endpoints.
type AnalyticsHandler struct {
	svc port.AnalyticsService
	log logger.Logger
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(svc port.AnalyticsService, log logger.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc, log: log}
}

// GetCapacity godoc
// @Summary      Get infrastructure capacity summary
// @Tags         analytics
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /analytics/capacity [get]
func (h *AnalyticsHandler) GetCapacity(c *gin.Context) {
	summary, err := h.svc.GetCapacitySummary(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, summary)
}

// GetCapacityTrends godoc
// @Summary      Get capacity trends (time-series)
// @Tags         analytics
// @Security     BearerAuth
// @Produce      json
// @Param        hypervisor_id  query  string  false  "Filter by hypervisor"
// @Param        since          query  string  false  "Start time (RFC3339)"
// @Param        granularity    query  string  false  "hour|day|week"
// @Success      200  {object}  APIResponse
// @Router       /analytics/capacity/trends [get]
func (h *AnalyticsHandler) GetCapacityTrends(c *gin.Context) {
	req := port.CapacityTrendsRequest{
		HypervisorID: c.Query("hypervisor_id"),
		Granularity:  c.DefaultQuery("granularity", "day"),
	}

	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := parseTime(sinceStr); err == nil {
			req.Since = t
		}
	}

	trends, err := h.svc.GetCapacityTrends(c.Request.Context(), req)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, trends)
}

// GetProviderCapacity godoc
// @Summary      Get per-provider capacity details
// @Tags         analytics
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /analytics/capacity/providers [get]
func (h *AnalyticsHandler) GetProviderCapacity(c *gin.Context) {
	caps, err := h.svc.GetProviderCapacity(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, caps)
}

// GetRecommendations godoc
// @Summary      Get optimization recommendations
// @Tags         analytics
// @Security     BearerAuth
// @Produce      json
// @Param        type          query  string  false  "Filter by type"
// @Param        severity      query  string  false  "Filter by severity"
// @Param        status        query  string  false  "Filter by status (default: active)"
// @Param        hypervisor_id query  string  false  "Filter by hypervisor"
// @Param        vm_id         query  string  false  "Filter by VM"
// @Param        page          query  int     false  "Page number"
// @Param        page_size     query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /analytics/recommendations [get]
func (h *AnalyticsHandler) GetRecommendations(c *gin.Context) {
	filter := port.RecommendationFilter{
		Type:         c.Query("type"),
		Severity:     c.Query("severity"),
		Status:       c.Query("status"),
		HypervisorID: c.Query("hypervisor_id"),
		VMID:         c.Query("vm_id"),
	}
	result, err := h.svc.GetRecommendations(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// GetRecommendationSummary godoc
// @Summary      Get recommendation counts by severity and type
// @Tags         analytics
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /analytics/recommendations/summary [get]
func (h *AnalyticsHandler) GetRecommendationSummary(c *gin.Context) {
	summary, err := h.svc.GetRecommendationSummary(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, summary)
}

type dismissRequest struct {
	Note string `json:"note"`
}

// DismissRecommendation godoc
// @Summary      Dismiss a recommendation
// @Tags         analytics
// @Security     BearerAuth
// @Param        id    path  string          true  "Recommendation ID"
// @Param        body  body  dismissRequest  false "Optional note"
// @Success      204
// @Router       /analytics/recommendations/{id}/dismiss [post]
func (h *AnalyticsHandler) DismissRecommendation(c *gin.Context) {
	var req dismissRequest
	_ = c.ShouldBindJSON(&req)

	userID := middleware.UserIDFromContext(c.Request.Context())
	if err := h.svc.DismissRecommendation(c.Request.Context(), c.Param("id"), userID, req.Note); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// ResolveRecommendation godoc
// @Summary      Mark a recommendation as resolved
// @Tags         analytics
// @Security     BearerAuth
// @Param        id  path  string  true  "Recommendation ID"
// @Success      204
// @Router       /analytics/recommendations/{id}/resolve [post]
func (h *AnalyticsHandler) ResolveRecommendation(c *gin.Context) {
	userID := middleware.UserIDFromContext(c.Request.Context())
	if err := h.svc.ResolveRecommendation(c.Request.Context(), c.Param("id"), userID); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// GetForecasts godoc
// @Summary      Get capacity forecasts
// @Tags         analytics
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /analytics/forecasting [get]
func (h *AnalyticsHandler) GetForecasts(c *gin.Context) {
	report, err := h.svc.GetForecasts(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, report)
}

// TriggerCollection godoc
// @Summary      Manually trigger a metrics collection cycle
// @Tags         analytics
// @Security     BearerAuth
// @Success      204
// @Router       /analytics/collect [post]
func (h *AnalyticsHandler) TriggerCollection(c *gin.Context) {
	go func() {
		ctx := c.Request.Context()
		if err := h.svc.CollectMetrics(ctx); err != nil {
			h.log.Warn("manual metrics collection failed", logger.Error(err))
		}
		if err := h.svc.RunOptimizationEngine(ctx); err != nil {
			h.log.Warn("manual optimization run failed", logger.Error(err))
		}
	}()
	noContent(c)
}
