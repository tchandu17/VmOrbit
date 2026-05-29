package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/vmOrbit/backend/internal/api/middleware"
	"github.com/vmOrbit/backend/internal/domain/port"
	ws "github.com/vmOrbit/backend/internal/websocket"
	"github.com/vmOrbit/backend/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Origin validation is handled by the CORS middleware.
		// For production, validate r.Header.Get("Origin") against an allowlist.
		return true
	},
}

// WSHandler upgrades HTTP connections to WebSocket.
type WSHandler struct {
	hub  *ws.Hub
	auth port.AuthService
	log  logger.Logger
}

// NewWSHandler creates a new WSHandler.
func NewWSHandler(hub *ws.Hub, auth port.AuthService, log logger.Logger) *WSHandler {
	return &WSHandler{hub: hub, auth: auth, log: log}
}

// Handle godoc
// @Summary      WebSocket endpoint
// @Description  Upgrade to WebSocket for real-time events. Send {"type":"subscribe","room":"tasks"} to subscribe to a room.
// @Tags         websocket
// @Security     BearerAuth
// @Router       /ws [get]
func (h *WSHandler) Handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("ws upgrade failed", logger.Error(err))
		return
	}

	userID := middleware.GetCurrentUserID(c)
	client := h.hub.Register(conn, userID)

	// Pump goroutines — each client gets its own pair
	go client.WritePump()
	go client.ReadPump()
}
