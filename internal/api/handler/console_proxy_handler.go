package handler

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ConsoleProxyHandler proxies WebSocket console connections to the hypervisor.
// This solves two problems:
//  1. Self-signed TLS certificates on ESXi/Proxmox — the backend already trusts them.
//  2. X-Frame-Options / CORS — the browser only talks to our trusted backend.
type ConsoleProxyHandler struct {
	svc port.ConsoleService
	log logger.Logger
}

// NewConsoleProxyHandler creates a new ConsoleProxyHandler.
func NewConsoleProxyHandler(svc port.ConsoleService, log logger.Logger) *ConsoleProxyHandler {
	return &ConsoleProxyHandler{svc: svc, log: log}
}

var consoleUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
	// Accept any subprotocol the client requests (e.g. "binary" for noVNC,
	// "vmware-wmks" for WebMKS).
	Subprotocols: []string{"binary", "vmware-wmks", "vmware-wmks+deflate", "base64"},
}

// ProxyConsole godoc
// @Summary      WebSocket console proxy
// @Description  Upgrades to WebSocket and proxies the console connection to the hypervisor.
// @Tags         console
// @Security     BearerAuth
// @Param        token  path  string  true  "Console session token"
// @Router       /consoles/{token}/ws [get]
// rejectWS upgrades the connection to WebSocket, sends an error message and a
// close frame, then closes. This gives the browser a proper close code and a
// human-readable reason instead of the opaque code=1006 that results from
// returning a plain HTTP error response to a WebSocket upgrade request.
func rejectWS(c *gin.Context, reason string) {
	if client, err := consoleUpgrader.Upgrade(c.Writer, c.Request, nil); err == nil {
		_ = client.WriteMessage(websocket.TextMessage, []byte(reason))
		_ = client.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, reason),
			time.Now().Add(2*time.Second),
		)
		client.Close() //nolint:errcheck
	}
}

func (h *ConsoleProxyHandler) ProxyConsole(c *gin.Context) {
	token := c.Param("token")

	session, err := h.svc.GetSession(c.Request.Context(), token)
	if err != nil {
		rejectWS(c, fmt.Sprintf("session not found: %v", err))
		return
	}

	// ESXi MKS (port 902) uses a raw TCP+TLS VMRC protocol, not WebSocket.
	// We proxy it differently: connect TCP to ESXi, do the VMRC auth handshake,
	// then bridge the raw VNC byte stream to the browser over WebSocket.
	if mksPort, ok := session.Extra["mks_port"].(float64); ok && int(mksPort) == 902 {
		h.proxyVMRC(c, session)
		return
	}

	// For all other types (novnc/Proxmox, webmks on port 443) use WS→WS proxy.
	h.proxyWebSocket(c, session)
}

// proxyVMRC handles ESXi standalone console sessions on port 902.
// ESXi port 902 is the VMware Authentication Daemon (VMRC protocol):
//   1. Plain TCP connect → ESXi sends banner: "220 VMware Authentication Daemon..."
//   2. Client upgrades to TLS (banner says "SSL Required")
//   3. Client sends: "USER <ticket>\r\n"  → ESXi: "331 Password required"
//   4. Client sends: "PASS <ticket>\r\n"  → ESXi: "230 User logged in"
//   5. Client sends: "CONNECT <cfgFile> mks\r\n" → ESXi: "200 Connect..."
//   6. From here the connection is raw VNC — bridge it to the browser WS.
func (h *ConsoleProxyHandler) proxyVMRC(c *gin.Context, session *model.ConsoleSession) {
	token := session.SessionToken
	ticket := session.ProviderTicket
	host, _ := session.Extra["host"].(string)
	cfgFile, _ := session.Extra["cfg_file"].(string)
	if host == "" {
		rejectWS(c, "esxi: missing host in session")
		return
	}
	if cfgFile == "" {
		rejectWS(c, "esxi: missing cfg_file in session — cannot CONNECT to VM")
		return
	}

	h.log.Info("console proxy: VMRC connect",
		logger.String("token", token),
		logger.String("host", host),
		logger.String("cfg_file", cfgFile),
	)

	addr := fmt.Sprintf("%s:902", host)

	// Step 1: plain TCP connect
	rawConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		rejectWS(c, fmt.Sprintf("cannot connect to ESXi port 902: %v", err))
		return
	}
	rawConn.SetDeadline(time.Now().Add(15 * time.Second)) //nolint:errcheck

	// Step 2: read banner on plain TCP
	banner, err := readVMRCLine(rawConn)
	if err != nil || len(banner) < 3 || banner[:3] != "220" {
		rawConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: unexpected banner: %q err=%v", banner, err))
		return
	}
	h.log.Info("console proxy: VMRC banner", logger.String("banner", banner))

	// Step 3: upgrade to TLS (banner says "SSL Required")
	tlsConn := tls.Client(rawConn, &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		ServerName:         host,
	})
	if err := tlsConn.Handshake(); err != nil {
		rawConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: TLS handshake failed: %v", err))
		return
	}

	// Step 4: USER <ticket>
	if err := writeVMRCLine(tlsConn, "USER "+ticket); err != nil {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: USER write failed: %v", err))
		return
	}
	resp, err := readVMRCLine(tlsConn)
	if err != nil {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: USER response error: %v", err))
		return
	}
	h.log.Info("console proxy: VMRC USER", logger.String("resp", resp))

	// Step 5: PASS <ticket>
	if err := writeVMRCLine(tlsConn, "PASS "+ticket); err != nil {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: PASS write failed: %v", err))
		return
	}
	resp, err = readVMRCLine(tlsConn)
	if err != nil {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: PASS response error: %v", err))
		return
	}
	h.log.Info("console proxy: VMRC PASS", logger.String("resp", resp))
	if len(resp) < 3 || resp[:3] != "230" {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: authentication failed: %s", resp))
		return
	}

	// Step 6: CONNECT <cfgFile> mks
	if err := writeVMRCLine(tlsConn, "CONNECT "+cfgFile+" mks"); err != nil {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: CONNECT write failed: %v", err))
		return
	}
	resp, err = readVMRCLine(tlsConn)
	if err != nil {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: CONNECT response error: %v", err))
		return
	}
	h.log.Info("console proxy: VMRC CONNECT", logger.String("resp", resp))
	if len(resp) < 3 || resp[:3] != "200" {
		tlsConn.Close()
		rejectWS(c, fmt.Sprintf("VMRC: CONNECT rejected: %s", resp))
		return
	}

	// Auth done — raw VNC stream begins. Clear the deadline.
	tlsConn.SetDeadline(time.Time{}) //nolint:errcheck

	h.log.Info("console proxy: VMRC handshake complete — bridging VNC to WebSocket",
		logger.String("token", token))

	// Upgrade the browser connection to WebSocket (binary, for noVNC).
	upgrader := websocket.Upgrader{
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
		Subprotocols:    []string{"binary"},
	}
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("console proxy: VMRC client WS upgrade failed", logger.Error(err))
		tlsConn.Close()
		return
	}
	defer ws.Close() //nolint:errcheck

	// Bridge: TLS conn ↔ WebSocket binary frames.
	errc := make(chan error, 2)

	// TLS → WebSocket
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := tlsConn.Read(buf)
			if n > 0 {
				if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					errc <- werr
					return
				}
			}
			if err != nil {
				errc <- err
				return
			}
		}
	}()

	// WebSocket → TLS
	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if _, werr := tlsConn.Write(data); werr != nil {
				errc <- werr
				return
			}
		}
	}()

	if err := <-errc; err != nil && !isWSCloseError(err) {
		h.log.Warn("console proxy: VMRC tunnel closed with error",
			logger.String("token", token), logger.Error(err))
	}
	h.log.Info("console proxy: VMRC tunnel closed", logger.String("token", token))
}

// proxyWebSocket handles WebSocket-to-WebSocket proxying (Proxmox noVNC, vCenter WebMKS).
func (h *ConsoleProxyHandler) proxyWebSocket(c *gin.Context, session *model.ConsoleSession) {
	token := session.SessionToken

	// Build the upstream WebSocket URL from the session's Extra fields.
	upstreamURL, subprotocol, err := buildUpstreamWSURL(session)
	if err != nil {
		h.log.Error("console proxy: cannot build upstream URL",
			logger.String("token", token),
			logger.Error(err),
		)
		rejectWS(c, fmt.Sprintf("cannot determine console endpoint: %v", err))
		return
	}

	h.log.Info("console proxy: connecting",
		logger.String("token", token),
		logger.String("upstream", upstreamURL),
		logger.String("subprotocol", subprotocol),
	)

	// Dial the upstream hypervisor WebSocket.
	// ESXi WebMKS requires the Host header to match the ESXi hostname/IP.
	// We also set TLSClientConfig.ServerName explicitly so TLS SNI works
	// correctly when connecting to an IP address.
	esxiHost, _, _ := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(upstreamURL, "wss://"), "ws://"))
	if esxiHost == "" {
		// fallback: strip scheme and path
		u, _ := url.Parse(upstreamURL)
		esxiHost = u.Hostname()
	}
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // hypervisors use self-signed certs
			ServerName:         esxiHost,
		},
		HandshakeTimeout: 15 * time.Second,
	}
	if subprotocol != "" {
		dialer.Subprotocols = []string{subprotocol}
	}

	upstreamHeaders := http.Header{}
	// Proxmox requires a Cookie header with the VNC ticket.
	if session.ConsoleType == "novnc" {
		if ticket, ok := session.Extra["ticket"].(string); ok && ticket != "" {
			upstreamHeaders.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", ticket))
		}
	}

	upstream, resp, err := dialer.Dial(upstreamURL, upstreamHeaders)
	if err != nil {
		body := ""
		if resp != nil {
			b, _ := io.ReadAll(resp.Body)
			body = string(b)
		}
		h.log.Error("console proxy: upstream dial failed",
			logger.String("url", upstreamURL),
			logger.String("body", body),
			logger.Error(err),
		)
		// Upgrade the browser connection first so we can send a proper WebSocket
		// close frame with a human-readable reason. Without this the browser
		// receives a plain HTTP 502 response to its Upgrade request, which
		// manifests as the opaque code=1006 "abnormal closure" in the console.
		errMsg := fmt.Sprintf("upstream console connection failed: %v", err)
		if body != "" {
			errMsg = fmt.Sprintf("%s — upstream response: %s", errMsg, body)
		}
		if client, upgradeErr := consoleUpgrader.Upgrade(c.Writer, c.Request, nil); upgradeErr == nil {
			_ = client.WriteMessage(websocket.TextMessage, []byte(errMsg))
			_ = client.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "upstream unreachable"),
				time.Now().Add(2*time.Second),
			)
			client.Close() //nolint:errcheck
		}
		return
	}
	defer upstream.Close() //nolint:errcheck

	// Negotiate subprotocol with the browser client.
	negotiated := upstream.Subprotocol()
	if negotiated == "" && subprotocol != "" {
		negotiated = subprotocol
	}
	upgraderCopy := consoleUpgrader
	if negotiated != "" {
		upgraderCopy.Subprotocols = []string{negotiated}
	}

	// Upgrade the browser connection.
	client, err := upgraderCopy.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("console proxy: client upgrade failed", logger.Error(err))
		return
	}
	defer client.Close() //nolint:errcheck

	h.log.Info("console proxy: tunnel established",
		logger.String("token", token),
		logger.String("subprotocol", negotiated),
	)

	// Bidirectional proxy — two goroutines, one per direction.
	errc := make(chan error, 2)

	go func() {
		errc <- proxyWSFrames(upstream, client)
	}()
	go func() {
		errc <- proxyWSFrames(client, upstream)
	}()

	// Wait for either side to close.
	if err := <-errc; err != nil && !isWSCloseError(err) {
		h.log.Warn("console proxy: tunnel closed with error",
			logger.String("token", token),
			logger.Error(err),
		)
	}
	h.log.Info("console proxy: tunnel closed", logger.String("token", token))
}

// proxyWSFrames copies WebSocket frames from src to dst verbatim.
func proxyWSFrames(dst, src *websocket.Conn) error {
	for {
		msgType, data, err := src.ReadMessage()
		if err != nil {
			return err
		}
		if err := dst.WriteMessage(msgType, data); err != nil {
			return err
		}
	}
}

// isWSCloseError returns true for normal WebSocket close conditions.
func isWSCloseError(err error) bool {
	return websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseAbnormalClosure,
	)
}

// readVMRCLine reads a CRLF-terminated line from the VMRC connection.
func readVMRCLine(conn net.Conn) (string, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		n, err := conn.Read(b)
		if n > 0 {
			buf = append(buf, b[0])
			if len(buf) >= 2 && buf[len(buf)-2] == '\r' && buf[len(buf)-1] == '\n' {
				return strings.TrimRight(string(buf), "\r\n"), nil
			}
		}
		if err != nil {
			return strings.TrimRight(string(buf), "\r\n"), err
		}
	}
}

// writeVMRCLine writes a CRLF-terminated line to the VMRC connection.
func writeVMRCLine(conn net.Conn, line string) error {
	_, err := fmt.Fprintf(conn, "%s\r\n", line)
	return err
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func buildUpstreamWSURL(session *model.ConsoleSession) (wsURL string, subprotocol string, err error) {
	extra := session.Extra

	switch session.ConsoleType {
	case "webmks":
		if wss, ok := extra["wss_url"].(string); ok && wss != "" {
			return wss, "vmware-wmks", nil
		}
		// Fallback: reconstruct from ticket + host in Extra.
		ticket, _ := extra["ticket"].(string)
		host, _ := extra["host"].(string)
		port := 443
		if p, ok := extra["port"].(float64); ok && p > 0 {
			port = int(p)
		}
		if ticket != "" && host != "" {
			return fmt.Sprintf("wss://%s:%d/ticket/%s", host, port, ticket), "vmware-wmks", nil
		}
		return "", "", fmt.Errorf("webmks session missing wss_url in extra")

	case "novnc":
		// Proxmox noVNC: build the WebSocket URL from node/vmid/ticket.
		node, _ := extra["node"].(string)
		vmid, _ := extra["vmid"].(float64) // JSON numbers decode as float64
		ticket, _ := extra["ticket"].(string)
		if ticket == "" {
			ticket = session.ProviderTicket
		}

		host, _ := extra["host"].(string)
		port := 8006
		if p, ok := extra["port"].(float64); ok && p > 0 {
			port = int(p)
		}

		if node == "" || vmid == 0 {
			return "", "", fmt.Errorf("novnc session missing node/vmid in extra")
		}

		vncPort := 5900
		if p, ok := extra["vnc_port"].(float64); ok && p > 0 {
			vncPort = int(p)
		}

		wsURL = fmt.Sprintf(
			"wss://%s:%d/api2/json/nodes/%s/qemu/%d/vncwebsocket?port=%d&vncticket=%s",
			host, port, node, int(vmid), vncPort, url.QueryEscape(ticket),
		)
		return wsURL, "binary", nil

	default:
		return "", "", fmt.Errorf("unsupported console type %q for WebSocket proxy", session.ConsoleType)
	}
}
