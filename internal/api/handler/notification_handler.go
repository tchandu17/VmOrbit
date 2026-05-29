package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// NotificationHandler handles notification channel, rule, and history endpoints.
type NotificationHandler struct {
	channels port.NotificationChannelService
	rules    port.NotificationRuleService
	history  port.NotificationHistoryService
	log      logger.Logger
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(
	channels port.NotificationChannelService,
	rules port.NotificationRuleService,
	history port.NotificationHistoryService,
	log logger.Logger,
) *NotificationHandler {
	return &NotificationHandler{
		channels: channels,
		rules:    rules,
		history:  history,
		log:      log,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Channels
// ─────────────────────────────────────────────────────────────────────────────

// ListChannels godoc
// @Summary      List notification channels
// @Tags         notifications
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /notification-channels [get]
func (h *NotificationHandler) ListChannels(c *gin.Context) {
	items, err := h.channels.List(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, items)
}

// CreateChannel godoc
// @Summary      Create a notification channel
// @Tags         notifications
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      201  {object}  APIResponse
// @Router       /notification-channels [post]
func (h *NotificationHandler) CreateChannel(c *gin.Context) {
	var req struct {
		Name        string                        `json:"name"        binding:"required"`
		Type        model.NotificationChannelType `json:"type"        binding:"required"`
		Description string                        `json:"description"`
		Enabled     *bool                         `json:"enabled"`
		Config      model.JSONMap                 `json:"config"      binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	ch, err := h.channels.Create(c.Request.Context(), port.CreateNotificationChannelRequest{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Enabled:     enabled,
		Config:      req.Config,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, ch)
}

// GetChannel godoc
// @Summary      Get a notification channel by ID
// @Tags         notifications
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  APIResponse
// @Router       /notification-channels/{id} [get]
func (h *NotificationHandler) GetChannel(c *gin.Context) {
	ch, err := h.channels.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, "channel not found")
		return
	}
	ok(c, ch)
}

// UpdateChannel godoc
// @Summary      Update a notification channel
// @Tags         notifications
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  APIResponse
// @Router       /notification-channels/{id} [put]
func (h *NotificationHandler) UpdateChannel(c *gin.Context) {
	var req struct {
		Name        *string       `json:"name"`
		Description *string       `json:"description"`
		Enabled     *bool         `json:"enabled"`
		Config      model.JSONMap `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	ch, err := h.channels.Update(c.Request.Context(), c.Param("id"), port.UpdateNotificationChannelRequest{
		Name:        req.Name,
		Description: req.Description,
		Enabled:     req.Enabled,
		Config:      req.Config,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, ch)
}

// DeleteChannel godoc
// @Summary      Delete a notification channel
// @Tags         notifications
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      204
// @Router       /notification-channels/{id} [delete]
func (h *NotificationHandler) DeleteChannel(c *gin.Context) {
	if err := h.channels.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// TestChannel godoc
// @Summary      Send a test notification to a channel
// @Tags         notifications
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  APIResponse
// @Router       /notification-channels/{id}/test [post]
func (h *NotificationHandler) TestChannel(c *gin.Context) {
	if err := h.channels.Test(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, gin.H{"message": "test notification sent successfully"})
}

// ─────────────────────────────────────────────────────────────────────────────
// Rules
// ─────────────────────────────────────────────────────────────────────────────

// ListRules godoc
// @Summary      List notification rules
// @Tags         notifications
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /notification-rules [get]
func (h *NotificationHandler) ListRules(c *gin.Context) {
	items, err := h.rules.List(c.Request.Context())
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, items)
}

// CreateRule godoc
// @Summary      Create a notification rule
// @Tags         notifications
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      201  {object}  APIResponse
// @Router       /notification-rules [post]
func (h *NotificationHandler) CreateRule(c *gin.Context) {
	var req struct {
		Name            string   `json:"name"       binding:"required"`
		Description     string   `json:"description"`
		ChannelID       string   `json:"channel_id" binding:"required"`
		EventTypes      []string `json:"event_types"`
		Severities      []string `json:"severities"`
		Providers       []string `json:"providers"`
		ThrottleSeconds int      `json:"throttle_seconds"`
		Enabled         *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule, err := h.rules.Create(c.Request.Context(), port.CreateNotificationRuleRequest{
		Name:            req.Name,
		Description:     req.Description,
		ChannelID:       req.ChannelID,
		EventTypes:      req.EventTypes,
		Severities:      req.Severities,
		Providers:       req.Providers,
		ThrottleSeconds: req.ThrottleSeconds,
		Enabled:         enabled,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	created(c, rule)
}

// GetRule godoc
// @Summary      Get a notification rule by ID
// @Tags         notifications
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  APIResponse
// @Router       /notification-rules/{id} [get]
func (h *NotificationHandler) GetRule(c *gin.Context) {
	rule, err := h.rules.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c, "rule not found")
		return
	}
	ok(c, rule)
}

// UpdateRule godoc
// @Summary      Update a notification rule
// @Tags         notifications
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  APIResponse
// @Router       /notification-rules/{id} [put]
func (h *NotificationHandler) UpdateRule(c *gin.Context) {
	var req struct {
		Name            *string  `json:"name"`
		Description     *string  `json:"description"`
		ChannelID       *string  `json:"channel_id"`
		EventTypes      []string `json:"event_types"`
		Severities      []string `json:"severities"`
		Providers       []string `json:"providers"`
		ThrottleSeconds *int     `json:"throttle_seconds"`
		Enabled         *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	rule, err := h.rules.Update(c.Request.Context(), c.Param("id"), port.UpdateNotificationRuleRequest{
		Name:            req.Name,
		Description:     req.Description,
		ChannelID:       req.ChannelID,
		EventTypes:      req.EventTypes,
		Severities:      req.Severities,
		Providers:       req.Providers,
		ThrottleSeconds: req.ThrottleSeconds,
		Enabled:         req.Enabled,
	})
	if err != nil {
		internalError(c, err.Error())
		return
	}
	ok(c, rule)
}

// DeleteRule godoc
// @Summary      Delete a notification rule
// @Tags         notifications
// @Security     BearerAuth
// @Param        id  path  string  true  "Rule ID"
// @Success      204
// @Router       /notification-rules/{id} [delete]
func (h *NotificationHandler) DeleteRule(c *gin.Context) {
	if err := h.rules.Delete(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err.Error())
		return
	}
	noContent(c)
}

// ─────────────────────────────────────────────────────────────────────────────
// History
// ─────────────────────────────────────────────────────────────────────────────

// ListHistory godoc
// @Summary      List notification delivery history
// @Tags         notifications
// @Security     BearerAuth
// @Produce      json
// @Param        rule_id    query  string  false  "Filter by rule ID"
// @Param        channel_id query  string  false  "Filter by channel ID"
// @Param        event_id   query  string  false  "Filter by event ID"
// @Param        status     query  string  false  "Filter by status"
// @Param        since      query  string  false  "Filter from timestamp (RFC3339)"
// @Param        until      query  string  false  "Filter to timestamp (RFC3339)"
// @Param        page       query  int     false  "Page number"
// @Param        page_size  query  int     false  "Page size"
// @Success      200  {object}  APIResponse
// @Router       /notification-history [get]
func (h *NotificationHandler) ListHistory(c *gin.Context) {
	filter := buildHistoryFilter(c)
	result, err := h.history.List(c.Request.Context(), filter, parsePage(c))
	if err != nil {
		internalError(c, err.Error())
		return
	}
	paginated(c, result)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildHistoryFilter(c *gin.Context) port.NotificationHistoryFilter {
	filter := port.NotificationHistoryFilter{
		Status: c.Query("status"),
	}
	if raw := c.Query("rule_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.RuleID = &id
		}
	}
	if raw := c.Query("channel_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.ChannelID = &id
		}
	}
	if raw := c.Query("event_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.EventID = &id
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
