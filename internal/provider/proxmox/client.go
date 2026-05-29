// Package proxmox implements the port.Provider interface for Proxmox VE.
//
// Architecture:
//
//	provider.go  – port.Provider + port.ConsoleProvider implementation
//	client.go    – Proxmox REST API client (API token auth, HTTP lifecycle)
//	mapper.go    – Proxmox API response → port.* DTO conversions
//	retry.go     – exponential-backoff retry helper
//	task.go      – Proxmox async task (UPID) polling
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Client
// ─────────────────────────────────────────────────────────────────────────────

// Client is a thin, self-contained Proxmox VE REST API client.
// It authenticates exclusively via API tokens (PVEAPIToken header) which are
// stateless and do not require session management.
//
// Thread-safety: Client is safe for concurrent use after construction.
type Client struct {
	baseURL    string       // e.g. "https://pve.example.com:8006/api2/json"
	tokenID    string       // e.g. "user@pam!mytoken"
	tokenValue string       // the UUID secret
	httpClient *http.Client // shared, configured once
}

// newClient constructs a Client from the given credentials.
//
// tokenID is the full Proxmox API token identifier in the form
// "<user>@<realm>!<tokenname>" (e.g. "root@pam!vmorbit").
// tokenValue is the UUID secret shown once at token creation time.
//
// If verifyTLS is false the client skips certificate verification — only
// appropriate for self-signed lab environments.
func newClient(host string, port int, tokenID, tokenValue string, verifyTLS bool, timeout time.Duration) *Client {
	if port == 0 {
		port = 8006
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !verifyTLS, //nolint:gosec // intentional for lab use
		},
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Client{
		baseURL:    fmt.Sprintf("https://%s:%d/api2/json", host, port),
		tokenID:    tokenID,
		tokenValue: tokenValue,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Low-level HTTP helpers
// ─────────────────────────────────────────────────────────────────────────────

// get performs a GET request and decodes the Proxmox envelope into out.
func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// post performs a POST request with form-encoded body and decodes the response.
func (c *Client) post(ctx context.Context, path string, params url.Values, out interface{}) error {
	var body io.Reader
	if params != nil {
		body = strings.NewReader(params.Encode())
	}
	return c.doWithBody(ctx, http.MethodPost, path, "application/x-www-form-urlencoded", body, out)
}

// delete performs a DELETE request.
func (c *Client) delete(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodDelete, path, nil, out)
}

// do builds and executes an HTTP request without a body.
func (c *Client) do(ctx context.Context, method, path string, _ io.Reader, out interface{}) error {
	return c.doWithBody(ctx, method, path, "", nil, out)
}

// doWithBody is the core request executor.
func (c *Client) doWithBody(ctx context.Context, method, path, contentType string, body io.Reader, out interface{}) error {
	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return fmt.Errorf("proxmox: build request %s %s: %w", method, path, err)
	}

	// API token authentication — stateless, no cookie/ticket needed.
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenValue))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("proxmox: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("proxmox: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, path, respBody)
	}

	if out == nil {
		return nil
	}

	// Proxmox wraps all responses in {"data": ...}.
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("proxmox: decode envelope for %s: %w", path, err)
	}
	if envelope.Data == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("proxmox: decode data for %s: %w", path, err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// API error handling
// ─────────────────────────────────────────────────────────────────────────────

// apiError represents a structured Proxmox API error.
type apiError struct {
	StatusCode int
	Path       string
	Message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("proxmox API error %d on %s: %s", e.StatusCode, e.Path, e.Message)
}

// isNotFound returns true for 404 responses.
func (e *apiError) isNotFound() bool { return e.StatusCode == http.StatusNotFound }

// isAuthError returns true for 401/403 responses.
func (e *apiError) isAuthError() bool {
	return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
}

// parseAPIError extracts the error message from a non-2xx Proxmox response.
func parseAPIError(statusCode int, path string, body []byte) *apiError {
	// Proxmox error envelope: {"errors": {"field": "msg"}, "message": "..."}
	var errEnvelope struct {
		Errors  map[string]string `json:"errors"`
		Message string            `json:"message"`
	}
	msg := http.StatusText(statusCode)
	if err := json.Unmarshal(body, &errEnvelope); err == nil {
		if errEnvelope.Message != "" {
			msg = errEnvelope.Message
		} else if len(errEnvelope.Errors) > 0 {
			parts := make([]string, 0, len(errEnvelope.Errors))
			for k, v := range errEnvelope.Errors {
				parts = append(parts, fmt.Sprintf("%s: %s", k, v))
			}
			msg = strings.Join(parts, "; ")
		}
	}
	return &apiError{StatusCode: statusCode, Path: path, Message: msg}
}

// ─────────────────────────────────────────────────────────────────────────────
// Proxmox API response types
// ─────────────────────────────────────────────────────────────────────────────

// pveVersion is the response from GET /api2/json/version.
type pveVersion struct {
	Version string `json:"version"`
	Release string `json:"release"`
	RepoID  string `json:"repoid"`
}

// pveNode is a single entry from GET /api2/json/nodes.
type pveNode struct {
	Node   string  `json:"node"`
	Status string  `json:"status"` // "online" | "offline"
	CPU    float64 `json:"cpu"`
	MaxCPU int     `json:"maxcpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
	Uptime int64   `json:"uptime"`
}

// pveResource is a single entry from GET /api2/json/cluster/resources.
type pveResource struct {
	ID       string  `json:"id"`       // e.g. "qemu/100"
	Type     string  `json:"type"`     // "qemu" | "lxc" | "storage" | "node"
	Node     string  `json:"node"`
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Status   string  `json:"status"` // "running" | "stopped" | "paused"
	CPU      float64 `json:"cpu"`
	MaxCPU   int     `json:"maxcpu"`
	Mem      int64   `json:"mem"`
	MaxMem   int64   `json:"maxmem"`
	Disk     int64   `json:"disk"`
	MaxDisk  int64   `json:"maxdisk"`
	NetIn    int64   `json:"netin"`
	NetOut   int64   `json:"netout"`
	DiskRead int64   `json:"diskread"`
	DiskWrite int64  `json:"diskwrite"`
	Uptime   int64   `json:"uptime"`
	Template int     `json:"template"` // 1 = is a template
	OSType   string  `json:"ostype"`
	// Storage-specific
	Storage    string  `json:"storage"`
	PluginType string  `json:"plugintype"`
	Shared     int     `json:"shared"`
	Content    string  `json:"content"`
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Avail      int64   `json:"avail"`
}

// pveVMStatus is the response from GET /api2/json/nodes/{node}/qemu/{vmid}/status/current.
type pveVMStatus struct {
	VMID      int     `json:"vmid"`
	Name      string  `json:"name"`
	Status    string  `json:"status"` // "running" | "stopped" | "paused"
	CPU       float64 `json:"cpu"`
	CPUs      int     `json:"cpus"`
	Mem       int64   `json:"mem"`
	MaxMem    int64   `json:"maxmem"`
	Disk      int64   `json:"disk"`
	MaxDisk   int64   `json:"maxdisk"`
	NetIn     int64   `json:"netin"`
	NetOut    int64   `json:"netout"`
	DiskRead  int64   `json:"diskread"`
	DiskWrite int64   `json:"diskwrite"`
	Uptime    int64   `json:"uptime"`
	QMPStatus string  `json:"qmpstatus"` // "running" | "paused" | "stopped"
	PID       int     `json:"pid"`
}

// pveVMConfig is the response from GET /api2/json/nodes/{node}/qemu/{vmid}/config.
type pveVMConfig struct {
	Name    string `json:"name"`
	OSType  string `json:"ostype"`  // "l26" | "win10" | etc.
	Memory  int    `json:"memory"`  // MB
	Cores   int    `json:"cores"`
	Sockets int    `json:"sockets"`
	// Network interfaces: net0, net1, ...
	Net0 string `json:"net0"`
	Net1 string `json:"net1"`
	// Disk interfaces: scsi0, virtio0, ide0, ...
	Scsi0   string `json:"scsi0"`
	Virtio0 string `json:"virtio0"`
	IDE2    string `json:"ide2"`
	// Agent
	Agent string `json:"agent"`
	// Description
	Description string `json:"description"`
}

// pveSnapshot is a single entry from GET /api2/json/nodes/{node}/qemu/{vmid}/snapshot.
type pveSnapshot struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SnapTime    float64 `json:"snaptime"` // Unix timestamp
	Parent      string  `json:"parent"`
	VMState     int     `json:"vmstate"` // 1 = memory snapshot
}

// pveStorage is a single entry from GET /api2/json/nodes/{node}/storage.
type pveStorage struct {
	Storage    string  `json:"storage"`
	Type       string  `json:"type"`
	Active     int     `json:"active"`
	Enabled    int     `json:"enabled"`
	Shared     int     `json:"shared"`
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Avail      int64   `json:"avail"`
	UsedFrac   float64 `json:"used_fraction"`
	Content    string  `json:"content"`
}

// pveNetwork is a single entry from GET /api2/json/nodes/{node}/network.
type pveNetwork struct {
	Iface   string `json:"iface"`
	Type    string `json:"type"`   // "bridge" | "eth" | "bond" | "vlan"
	Active  int    `json:"active"`
	Bridge  string `json:"bridge_ports"`
	CIDR    string `json:"cidr"`
	Address string `json:"address"`
}

// flexInt unmarshals a JSON value that Proxmox may return as either a number
// or a quoted string (e.g. port is "5900" in some PVE versions, 5900 in others).
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	// Try plain number first.
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	// Fall back to quoted string.
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("flexInt: cannot parse %q as int: %w", s, err)
	}
	*f = flexInt(n)
	return nil
}

// pveVNCProxy is the response from POST /api2/json/nodes/{node}/qemu/{vmid}/vncproxy.
// NOTE: Proxmox returns "port" as a JSON string (e.g. "5900"), not an integer.
type pveVNCProxy struct {
	Ticket   string    `json:"ticket"`
	Port     flexInt   `json:"port"`
	Cert     string    `json:"cert"`
	User     string    `json:"user"`
	Password string    `json:"password"`
}

// pveTaskStatus is the response from GET /api2/json/nodes/{node}/tasks/{upid}/status.
type pveTaskStatus struct {
	Node      string  `json:"node"`
	UPID      string  `json:"upid"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`    // "running" | "stopped"
	ExitStatus string `json:"exitstatus"` // "OK" | error message
	StartTime float64 `json:"starttime"`
	EndTime   float64 `json:"endtime"`
	PID       int     `json:"pid"`
}

// pveRRDData is a single RRD data point from GET /api2/json/nodes/{node}/qemu/{vmid}/rrddata.
type pveRRDData struct {
	Time      float64 `json:"time"`
	CPU       float64 `json:"cpu"`
	Mem       float64 `json:"mem"`
	MaxMem    float64 `json:"maxmem"`
	NetIn     float64 `json:"netin"`
	NetOut    float64 `json:"netout"`
	DiskRead  float64 `json:"diskread"`
	DiskWrite float64 `json:"diskwrite"`
}

// ─────────────────────────────────────────────────────────────────────────────
// High-level API methods
// ─────────────────────────────────────────────────────────────────────────────

// GetVersion fetches the Proxmox VE version (used for Ping).
func (c *Client) GetVersion(ctx context.Context) (*pveVersion, error) {
	var v pveVersion
	if err := c.get(ctx, "/version", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// ListNodes returns all cluster nodes.
func (c *Client) ListNodes(ctx context.Context) ([]pveNode, error) {
	var nodes []pveNode
	if err := c.get(ctx, "/nodes", &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// ListClusterResources returns all cluster resources, optionally filtered by type.
// type can be "vm", "storage", "node", or "" for all.
func (c *Client) ListClusterResources(ctx context.Context, resType string) ([]pveResource, error) {
	path := "/cluster/resources"
	if resType != "" {
		path += "?type=" + url.QueryEscape(resType)
	}
	var resources []pveResource
	if err := c.get(ctx, path, &resources); err != nil {
		return nil, err
	}
	return resources, nil
}

// GetVMStatus fetches the current status of a QEMU VM.
func (c *Client) GetVMStatus(ctx context.Context, node string, vmid int) (*pveVMStatus, error) {
	var status pveVMStatus
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid)
	if err := c.get(ctx, path, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// GetVMConfig fetches the configuration of a QEMU VM.
func (c *Client) GetVMConfig(ctx context.Context, node string, vmid int) (*pveVMConfig, error) {
	var cfg pveVMConfig
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)
	if err := c.get(ctx, path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// VMPowerAction sends a power action to a QEMU VM and returns the UPID task ID.
// action is one of: "start", "stop", "reboot", "suspend", "reset", "shutdown".
func (c *Client) VMPowerAction(ctx context.Context, node string, vmid int, action string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/%s", node, vmid, action)
	var upid string
	if err := c.post(ctx, path, nil, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// CreateVM creates a new QEMU VM and returns the UPID task ID.
func (c *Client) CreateVM(ctx context.Context, node string, params url.Values) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", node)
	var upid string
	if err := c.post(ctx, path, params, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// DeleteVM destroys a QEMU VM and returns the UPID task ID.
// purge=true also removes the VM from the cluster-wide configuration.
func (c *Client) DeleteVM(ctx context.Context, node string, vmid int, purge bool) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", node, vmid)
	if purge {
		path += "?purge=1&destroy-unreferenced-disks=1"
	}
	var upid string
	if err := c.delete(ctx, path, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// CloneVM clones a QEMU VM and returns the UPID task ID.
func (c *Client) CloneVM(ctx context.Context, node string, vmid int, params url.Values) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", node, vmid)
	var upid string
	if err := c.post(ctx, path, params, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// ListSnapshots returns all snapshots for a QEMU VM.
func (c *Client) ListSnapshots(ctx context.Context, node string, vmid int) ([]pveSnapshot, error) {
	var snaps []pveSnapshot
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
	if err := c.get(ctx, path, &snaps); err != nil {
		return nil, err
	}
	return snaps, nil
}

// CreateSnapshot creates a snapshot and returns the UPID task ID.
func (c *Client) CreateSnapshot(ctx context.Context, node string, vmid int, params url.Values) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
	var upid string
	if err := c.post(ctx, path, params, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// DeleteSnapshot deletes a snapshot and returns the UPID task ID.
func (c *Client) DeleteSnapshot(ctx context.Context, node string, vmid int, snapname string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s", node, vmid, url.PathEscape(snapname))
	var upid string
	if err := c.delete(ctx, path, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// RollbackSnapshot reverts a VM to a snapshot and returns the UPID task ID.
func (c *Client) RollbackSnapshot(ctx context.Context, node string, vmid int, snapname string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s/rollback", node, vmid, url.PathEscape(snapname))
	var upid string
	if err := c.post(ctx, path, nil, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// ListStorage returns all storage pools on a node.
func (c *Client) ListStorage(ctx context.Context, node string) ([]pveStorage, error) {
	var stores []pveStorage
	path := fmt.Sprintf("/nodes/%s/storage", node)
	if err := c.get(ctx, path, &stores); err != nil {
		return nil, err
	}
	return stores, nil
}

// ListNetworks returns all network interfaces on a node.
func (c *Client) ListNetworks(ctx context.Context, node string) ([]pveNetwork, error) {
	var nets []pveNetwork
	path := fmt.Sprintf("/nodes/%s/network", node)
	if err := c.get(ctx, path, &nets); err != nil {
		return nil, err
	}
	return nets, nil
}

// VNCProxy creates a noVNC proxy session for a VM and returns the ticket.
func (c *Client) VNCProxy(ctx context.Context, node string, vmid int, websocket bool) (*pveVNCProxy, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", node, vmid)
	params := url.Values{}
	if websocket {
		params.Set("websocket", "1")
	}
	var proxy pveVNCProxy
	if err := c.post(ctx, path, params, &proxy); err != nil {
		return nil, err
	}
	return &proxy, nil
}

// GetTaskStatus fetches the current status of a Proxmox task by UPID.
func (c *Client) GetTaskStatus(ctx context.Context, node, upid string) (*pveTaskStatus, error) {
	var status pveTaskStatus
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, url.PathEscape(upid))
	if err := c.get(ctx, path, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// GetVMRRDData fetches RRD performance data for a VM.
// timeframe is one of: "hour", "day", "week", "month", "year".
func (c *Client) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe string) ([]pveRRDData, error) {
	if timeframe == "" {
		timeframe = "hour"
	}
	path := fmt.Sprintf("/nodes/%s/qemu/%d/rrddata?timeframe=%s&cf=AVERAGE",
		node, vmid, url.QueryEscape(timeframe))
	var data []pveRRDData
	if err := c.get(ctx, path, &data); err != nil {
		return nil, err
	}
	return data, nil
}
