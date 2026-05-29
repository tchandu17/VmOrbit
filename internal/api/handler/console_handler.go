package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ConsoleHandler handles console session REST endpoints.
type ConsoleHandler struct {
	svc port.ConsoleService
	log logger.Logger
}

// NewConsoleHandler creates a new ConsoleHandler.
func NewConsoleHandler(svc port.ConsoleService, log logger.Logger) *ConsoleHandler {
	return &ConsoleHandler{svc: svc, log: log}
}

type requestConsoleRequest struct {
	Type       string `json:"type"`
	TTLSeconds int    `json:"ttl_seconds"`
}

// consoleSessionResponse is the JSON shape returned to the frontend.
type consoleSessionResponse struct {
	SessionID   string `json:"session_id"`
	Token       string `json:"token"`
	ConsoleType string `json:"console_type"`
	URL         string `json:"url"`
	ProxyWSURL  string `json:"proxy_ws_url"`
	DirectWSURL string `json:"direct_ws_url,omitempty"`
	// CfgFile is the VM config file path used for VMRC CONNECT (debug/info only).
	CfgFile   string `json:"cfg_file,omitempty"`
	ExpiresAt string `json:"expires_at"`
	Provider  string `json:"provider"`
}

func buildResponse(session *model.ConsoleSession) consoleSessionResponse {
	resp := consoleSessionResponse{
		SessionID:   session.ID.String(),
		Token:       session.SessionToken,
		ConsoleType: session.ConsoleType,
		URL:         session.ConsoleURL,
		ProxyWSURL:  fmt.Sprintf("/api/v1/consoles/%s/ws", session.SessionToken),
		ExpiresAt:   session.ExpiresAt.Format(time.RFC3339),
		Provider:    string(session.Provider),
	}
	// Expose the raw wss_url for webmks so the frontend can connect directly
	// when the backend proxy fails (standalone ESXi rejects cross-origin upgrades).
	if wss, ok := session.Extra["wss_url"].(string); ok && wss != "" {
		resp.DirectWSURL = wss
	}
	if cf, ok := session.Extra["cfg_file"].(string); ok && cf != "" {
		resp.CfgFile = cf
	}
	return resp
}

// RequestSession godoc
// @Summary      Request a console session for a VM
// @Tags         console
// @Router       /vms/{id}/console [post]
func (h *ConsoleHandler) RequestSession(c *gin.Context) {
	var req requestConsoleRequest
	_ = c.ShouldBindJSON(&req)

	opts := port.ConsoleOptions{Type: port.ConsoleType(req.Type)}
	if req.TTLSeconds > 0 {
		opts.TTL = time.Duration(req.TTLSeconds) * time.Second
	}

	session, err := h.svc.RequestSession(c.Request.Context(), c.Param("id"), opts)
	if err != nil {
		internalError(c, err.Error())
		return
	}
	if session == nil {
		internalError(c, "provider returned empty console session")
		return
	}

	h.log.Info("console session created",
		logger.String("session_id", session.ID.String()),
		logger.String("console_type", session.ConsoleType),
		logger.String("provider", string(session.Provider)),
		logger.String("url_len", fmt.Sprintf("%d", len(session.ConsoleURL))),
	)

	ok(c, buildResponse(session))
}

// GetSession godoc
// @Summary      Get a console session by token
// @Tags         console
// @Router       /consoles/{token} [get]
func (h *ConsoleHandler) GetSession(c *gin.Context) {
	session, err := h.svc.GetSession(c.Request.Context(), c.Param("token"))
	if err != nil {
		notFound(c, err.Error())
		return
	}
	ok(c, buildResponse(session))
}
