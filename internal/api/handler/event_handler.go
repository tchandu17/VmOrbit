package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// EventHandler handles platform event REST endpoints.
type EventHandler struct {
	svc port.PlatformEventService
	log logger.Logger
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(svc port.PlatformEventService, log logger.Logger) *EventHandler {
	return &EventHandler{svc: svc, log: log}
}

// List godoc
// @Summary      List platform events
// @Tags         events
// @Security     BearerAuth
// @Produce      json
// @Param        event_type    query  string  false  "Filter by event type"
// @Param        severity      query  string  false  "Filter by severity (info|warning|critical)"
// @Param        provider      query  string  false  "Filter by provider"
// @Param        resource_type query  string  false  "Filter by resource type"
// @Param        resource_id   query  string  false  "Filter by resource ID (UUID)"
// @Param        hypervisor_id query  string  false  "Filter by hypervisor ID (UUID)"
// @Param        since         query  string  false  "Filter from timestamp (RFC3339)"
// @Param        until         query  string  false  "Filter to timestamp (RFC3339)"
// @Param        search        query  string  false  "Search in message"
// @Param        page          query  int     false  "Page number"
// @Param        page_size     query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /events [get]
func (h *EventHandler) List(c *gin.Context) {
	filter := buildEventFilter(c)
	result, err := h.svc.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// GetByID godoc
// @Summary      Get a platform event by ID
// @Tags         events
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Event ID"
// @Success      200  {object}  APIResponse
// @Router       /events/{id} [get]
func (h *EventHandler) GetByID(c *gin.Context) {
	event, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, "event not found")
		return
	}
	ok(c, event)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildEventFilter(c *gin.Context) port.PlatformEventFilter {
	filter := port.PlatformEventFilter{
		EventType:    c.Query("event_type"),
		Severity:     c.Query("severity"),
		Provider:     c.Query("provider"),
		ResourceType: c.Query("resource_type"),
		Search:       c.Query("search"),
	}

	if raw := c.Query("resource_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.ResourceID = &id
		}
	}
	if raw := c.Query("hypervisor_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.HypervisorID = &id
		}
	}
	if raw := c.Query("since"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			filter.Since = &t
		}
	}
	if raw := c.Query("until"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			filter.Until = &t
		}
	}

	return filter
}
