package handler

// ─────────────────────────────────────────────────────────────────────────────
// MaintenanceHandler — maintenance mode middleware and toggle endpoint
//
// When maintenance mode is active:
//   - All API requests return 503 with a Retry-After header
//   - /health, /ready, /status probes are unaffected
//   - Authenticated admins can still reach /api/v1/system/maintenance
// ─────────────────────────────────────────────────────────────────────────────

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// MaintenanceState is a shared atomic flag for maintenance mode.
// It is safe to read from multiple goroutines without locking.
type MaintenanceState struct {
	active    atomic.Bool
	activeSince time.Time
	reason    atomic.Value // stores string
}

// NewMaintenanceState creates a new MaintenanceState (off by default).
func NewMaintenanceState() *MaintenanceState {
	return &MaintenanceState{}
}

// Enable activates maintenance mode with an optional reason.
func (m *MaintenanceState) Enable(reason string) {
	m.activeSince = time.Now()
	m.reason.Store(reason)
	m.active.Store(true)
}

// Disable deactivates maintenance mode.
func (m *MaintenanceState) Disable() {
	m.active.Store(false)
	m.reason.Store("")
}

// IsActive returns true when maintenance mode is on.
func (m *MaintenanceState) IsActive() bool {
	return m.active.Load()
}

// Reason returns the current maintenance reason string.
func (m *MaintenanceState) Reason() string {
	if v := m.reason.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Middleware
// ─────────────────────────────────────────────────────────────────────────────

// MaintenanceMiddleware returns a Gin middleware that blocks requests when
// maintenance mode is active. Probe paths are always allowed through.
func MaintenanceMiddleware(state *MaintenanceState) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !state.IsActive() {
			c.Next()
			return
		}

		// Always allow probe endpoints
		path := c.Request.URL.Path
		if path == "/health" || path == "/ready" || path == "/status" || path == "/metrics" {
			c.Next()
			return
		}

		reason := state.Reason()
		if reason == "" {
			reason = "Scheduled maintenance in progress"
		}

		c.Header("Retry-After", "300")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":        "service_unavailable",
			"message":      reason,
			"maintenance":  true,
			"retry_after":  300,
		})
		c.Abort()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Handler
// ─────────────────────────────────────────────────────────────────────────────

// MaintenanceHandler exposes endpoints to toggle maintenance mode.
type MaintenanceHandler struct {
	state *MaintenanceState
}

// NewMaintenanceHandler creates a new MaintenanceHandler.
func NewMaintenanceHandler(state *MaintenanceState) *MaintenanceHandler {
	return &MaintenanceHandler{state: state}
}

// GetStatus returns the current maintenance mode status.
func (h *MaintenanceHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"maintenance": h.state.IsActive(),
		"reason":      h.state.Reason(),
		"since":       h.state.activeSince,
	})
}

// Enable activates maintenance mode.
// POST /api/v1/system/maintenance/enable
// Body: {"reason": "Upgrading database schema"}
func (h *MaintenanceHandler) Enable(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Reason == "" {
		req.Reason = "Maintenance in progress"
	}
	h.state.Enable(req.Reason)
	c.JSON(http.StatusOK, gin.H{
		"maintenance": true,
		"reason":      req.Reason,
		"message":     "Maintenance mode enabled",
	})
}

// Disable deactivates maintenance mode.
// POST /api/v1/system/maintenance/disable
func (h *MaintenanceHandler) Disable(c *gin.Context) {
	h.state.Disable()
	c.JSON(http.StatusOK, gin.H{
		"maintenance": false,
		"message":     "Maintenance mode disabled",
	})
}
