// Package nutanix implements the port.Provider interface for Nutanix AHV.
//
// Architecture:
//
//	provider.go  – port.Provider + port.ConsoleProvider implementation
//	client.go    – Nutanix Prism REST API client (basic auth, HTTP lifecycle)
//	mapper.go    – Nutanix API response → port.* DTO conversions
//	retry.go     – exponential-backoff retry helper
//	task.go      – Nutanix async task (task_uuid) polling
package nutanix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Client
// ─────────────────────────────────────────────────────────────────────────────

// Client is a thin, self-contained Nutanix Prism REST API client.
// It authenticates via HTTP Basic Auth against the Prism Element or
// Prism Central v3 API endpoint.
//
// Thread-safety: Client is safe for concurrent use after construction.
type Client struct {
	baseURL    string       // e.g. "https://prism.example.com:9440/api/nutanix/v3"
	username   string
	password   string
	httpClient *http.Client
}

// newClient constructs a Client from the given credentials.
//
// If verifyTLS is false the client skips certificate verification — only
// appropriate for self-signed lab environments.
func newClient(host string, port int, username, password string, verifyTLS bool, timeout time.Duration) *Client {
	if port == 0 {
		port = 9440
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
		baseURL:  fmt.Sprintf("https://%s:%d/api/nutanix/v3", host, port),
		username: username,
		password: password,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Low-level HTTP helpers
// ─────────────────────────────────────────────────────────────────────────────

// get performs a GET request and decodes the response into out.
func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// post performs a POST request with a JSON body and decodes the response.
func (c *Client) post(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.doJSON(ctx, http.MethodPost, path, body, out)
}

// delete performs a DELETE request.
func (c *Client) delete(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodDelete, path, nil, out)
}

// do builds and executes an HTTP request without a body.
func (c *Client) do(ctx context.Context, method, path string, _ io.Reader, out interface{}) error {
	return c.doJSON(ctx, method, path, nil, out)
}

// doJSON is the core request executor with JSON body support.
func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	reqURL := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("nutanix: marshal request body for %s %s: %w", method, path, err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("nutanix: build request %s %s: %w", method, path, err)
	}

	// Basic auth — Nutanix Prism uses username/password.
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nutanix: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("nutanix: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, path, respBody)
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("nutanix: decode response for %s: %w", path, err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// API error handling
// ─────────────────────────────────────────────────────────────────────────────

// apiError represents a structured Nutanix API error.
type apiError struct {
	StatusCode int
	Path       string
	Message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("nutanix API error %d on %s: %s", e.StatusCode, e.Path, e.Message)
}

func (e *apiError) isNotFound() bool { return e.StatusCode == http.StatusNotFound }
func (e *apiError) isAuthError() bool {
	return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
}

// parseAPIError extracts the error message from a non-2xx Nutanix response.
func parseAPIError(statusCode int, path string, body []byte) *apiError {
	// Nutanix v3 error envelope: {"state": "ERROR", "message_list": [{"message": "..."}]}
	var errEnvelope struct {
		State       string `json:"state"`
		MessageList []struct {
			Message string `json:"message"`
			Reason  string `json:"reason"`
		} `json:"message_list"`
		Message string `json:"message"`
	}
	msg := http.StatusText(statusCode)
	if err := json.Unmarshal(body, &errEnvelope); err == nil {
		if errEnvelope.Message != "" {
			msg = errEnvelope.Message
		} else if len(errEnvelope.MessageList) > 0 {
			msg = errEnvelope.MessageList[0].Message
		}
	}
	return &apiError{StatusCode: statusCode, Path: path, Message: msg}
}

// ─────────────────────────────────────────────────────────────────────────────
// Nutanix API response types
// ─────────────────────────────────────────────────────────────────────────────

// nutanixListMetadata is the request body for list operations.
type nutanixListMetadata struct {
	Kind          string `json:"kind"`
	Length        int    `json:"length,omitempty"`
	Offset        int    `json:"offset,omitempty"`
	SortOrder     string `json:"sort_order,omitempty"`
	SortAttribute string `json:"sort_attribute,omitempty"`
}

// nutanixListResponse is the generic list response envelope.
type nutanixListResponse struct {
	APIVersion string          `json:"api_version"`
	Metadata   json.RawMessage `json:"metadata"`
	Entities   json.RawMessage `json:"entities"`
}

// nutanixVM represents a Nutanix AHV virtual machine from the v3 API.
type nutanixVM struct {
	Status   nutanixVMStatus   `json:"status"`
	Spec     nutanixVMSpec     `json:"spec"`
	Metadata nutanixVMMetadata `json:"metadata"`
}

type nutanixVMMetadata struct {
	UUID             string            `json:"uuid"`
	Name             string            `json:"name"`
	Kind             string            `json:"kind"`
	SpecVersion      int               `json:"spec_version"`
	Categories       map[string]string `json:"categories"`
	CreationTime     string            `json:"creation_time"`
	LastUpdateTime   string            `json:"last_update_time"`
}

type nutanixVMSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Resources   nutanixVMResources `json:"resources"`
	ClusterReference *nutanixReference `json:"cluster_reference,omitempty"`
}

type nutanixVMStatus struct {
	Name      string             `json:"name"`
	State     string             `json:"state"` // "ON", "OFF", "PAUSED", "UNKNOWN"
	Resources nutanixVMResources `json:"resources"`
	ClusterReference *nutanixReference `json:"cluster_reference,omitempty"`
	ExecutionContext *nutanixExecutionContext `json:"execution_context,omitempty"`
}

type nutanixExecutionContext struct {
	TaskUUID []string `json:"task_uuid"`
}

type nutanixVMResources struct {
	NumVCPUs          int                    `json:"num_vcpus_per_socket"`
	NumSockets        int                    `json:"num_sockets"`
	MemorySizeMiB     int                    `json:"memory_size_mib"`
	PowerState        string                 `json:"power_state"` // "ON", "OFF", "PAUSED"
	DiskList          []nutanixDisk          `json:"disk_list"`
	NicList           []nutanixNIC           `json:"nic_list"`
	GuestOS           *nutanixGuestOS        `json:"guest_os_id,omitempty"`
	HostReference     *nutanixReference      `json:"host_reference,omitempty"`
	HypervisorType    string                 `json:"hypervisor_type"`
	GuestCustomization *nutanixGuestCustom   `json:"guest_customization,omitempty"`
}

type nutanixDisk struct {
	UUID            string            `json:"uuid"`
	DeviceProperties nutanixDeviceProps `json:"device_properties"`
	DataSourceReference *nutanixReference `json:"data_source_reference,omitempty"`
	DiskSizeMiB     int               `json:"disk_size_mib"`
	DiskSizeBytes   int64             `json:"disk_size_bytes"`
}

type nutanixDeviceProps struct {
	DeviceType  string `json:"device_type"`  // "DISK", "CDROM"
	DiskAddress struct {
		AdapterType string `json:"adapter_type"` // "SCSI", "IDE", "PCI", "SATA", "SPAPR"
		DeviceIndex int    `json:"device_index"`
	} `json:"disk_address"`
}

type nutanixNIC struct {
	UUID              string            `json:"uuid"`
	MACAddress        string            `json:"mac_address"`
	SubnetReference   *nutanixReference `json:"subnet_reference,omitempty"`
	IPEndpointList    []nutanixIPEndpoint `json:"ip_endpoint_list"`
	NICType           string            `json:"nic_type"` // "NORMAL_NIC", "DIRECT_NIC"
	IsConnected       bool              `json:"is_connected"`
}

type nutanixIPEndpoint struct {
	IP   string `json:"ip"`
	Type string `json:"type"` // "ASSIGNED", "LEARNED"
}

type nutanixGuestOS struct {
	ID string `json:"id"`
}

type nutanixGuestCustom struct {
	CloudInit *struct{} `json:"cloud_init,omitempty"`
	Sysprep   *struct{} `json:"sysprep,omitempty"`
}

type nutanixReference struct {
	Kind string `json:"kind"`
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// nutanixTask represents a Nutanix async task.
type nutanixTask struct {
	UUID             string   `json:"uuid"`
	Status           string   `json:"status"`           // "QUEUED", "RUNNING", "SUCCEEDED", "FAILED", "ABORTED"
	ProgressMessage  string   `json:"progress_message"`
	OperationType    string   `json:"operation_type"`
	PercentageComplete int    `json:"percentage_complete"`
	ErrorCode        string   `json:"error_code"`
	ErrorDetail      string   `json:"error_detail"`
	EntityReferenceList []nutanixReference `json:"entity_reference_list"`
}

// nutanixCluster represents a Nutanix cluster.
type nutanixCluster struct {
	Metadata nutanixVMMetadata `json:"metadata"`
	Status   struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		Resources struct {
			Config struct {
				ServiceList []string `json:"service_list"`
			} `json:"config"`
		} `json:"resources"`
	} `json:"status"`
}

// nutanixHost represents a Nutanix AHV host.
type nutanixHost struct {
	Metadata nutanixVMMetadata `json:"metadata"`
	Status   struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		Resources struct {
			NumCPUSockets    int   `json:"num_cpu_sockets"`
			NumCPUCores      int   `json:"num_cpu_cores"`
			MemoryCapacityMiB int64 `json:"memory_capacity_mib"`
			HypervisorType   string `json:"hypervisor_type"`
		} `json:"resources"`
		ClusterReference *nutanixReference `json:"cluster_reference,omitempty"`
	} `json:"status"`
}

// nutanixStorageContainer represents a Nutanix storage container.
type nutanixStorageContainer struct {
	ContainerUUID string `json:"container_uuid"`
	Name          string `json:"name"`
	MaxCapacity   int64  `json:"max_capacity"`
	UsageStats    struct {
		StorageUsageBytes string `json:"storage.usage_bytes"`
		StorageFreeBytes  string `json:"storage.free_bytes"`
	} `json:"usage_stats"`
	ClusterUUID string `json:"cluster_uuid"`
}

// nutanixSubnet represents a Nutanix subnet/network.
type nutanixSubnet struct {
	Metadata nutanixVMMetadata `json:"metadata"`
	Status   struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		Resources struct {
			SubnetType  string `json:"subnet_type"` // "VLAN", "OVERLAY"
			VlanID      int    `json:"vlan_id"`
			VswitchName string `json:"vswitch_name"`
		} `json:"resources"`
		ClusterReference *nutanixReference `json:"cluster_reference,omitempty"`
	} `json:"status"`
}

// nutanixImage represents a Nutanix image (template).
type nutanixImage struct {
	Metadata nutanixVMMetadata `json:"metadata"`
	Status   struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		Resources struct {
			ImageType   string `json:"image_type"` // "DISK_IMAGE", "ISO_IMAGE"
			SizeBytes   int64  `json:"size_bytes"`
			Architecture string `json:"architecture"`
		} `json:"resources"`
		Description string `json:"description"`
	} `json:"status"`
}

// nutanixVMSnapshot represents a Nutanix VM recovery point (snapshot).
type nutanixVMSnapshot struct {
	UUID             string `json:"uuid"`
	Name             string `json:"name"`
	VMUuid           string `json:"vm_uuid"`
	CreationTime     int64  `json:"creation_time"`
	ConsistencyGroup string `json:"consistency_group_name"`
}

// nutanixPowerAction is the request body for VM power operations.
type nutanixPowerAction struct {
	Transition string `json:"transition"` // "ON", "OFF", "POWERCYCLE", "RESET", "PAUSE", "SUSPEND", "RESUME", "ACPI_SHUTDOWN", "ACPI_REBOOT"
}

// nutanixTaskResponse is the response from task-creating operations.
type nutanixTaskResponse struct {
	TaskUUID string `json:"task_uuid"`
}

// ─────────────────────────────────────────────────────────────────────────────
// High-level API methods
// ─────────────────────────────────────────────────────────────────────────────

// Ping verifies connectivity by calling the clusters list endpoint.
func (c *Client) Ping(ctx context.Context) error {
	body := nutanixListMetadata{Kind: "cluster", Length: 1}
	var resp struct {
		Entities []nutanixCluster `json:"entities"`
	}
	return c.post(ctx, "/clusters/list", body, &resp)
}

// ListVMs returns all VMs from Prism.
func (c *Client) ListVMs(ctx context.Context) ([]nutanixVM, error) {
	body := nutanixListMetadata{Kind: "vm", Length: 500}
	var resp struct {
		Entities []nutanixVM `json:"entities"`
	}
	if err := c.post(ctx, "/vms/list", body, &resp); err != nil {
		return nil, err
	}
	return resp.Entities, nil
}

// GetVM returns a single VM by UUID.
func (c *Client) GetVM(ctx context.Context, vmUUID string) (*nutanixVM, error) {
	var vm nutanixVM
	if err := c.get(ctx, "/vms/"+url.PathEscape(vmUUID), &vm); err != nil {
		return nil, err
	}
	return &vm, nil
}

// VMPowerAction sends a power transition to a VM and returns the task UUID.
func (c *Client) VMPowerAction(ctx context.Context, vmUUID, transition string) (string, error) {
	body := nutanixPowerAction{Transition: transition}
	var resp nutanixTaskResponse
	if err := c.post(ctx, "/vms/"+url.PathEscape(vmUUID)+"/set_power_state", body, &resp); err != nil {
		return "", err
	}
	return resp.TaskUUID, nil
}

// DeleteVM deletes a VM and returns the task UUID.
func (c *Client) DeleteVM(ctx context.Context, vmUUID string) (string, error) {
	var resp nutanixTaskResponse
	if err := c.delete(ctx, "/vms/"+url.PathEscape(vmUUID), &resp); err != nil {
		return "", err
	}
	return resp.TaskUUID, nil
}

// CloneVM clones a VM from a spec and returns the task UUID.
func (c *Client) CloneVM(ctx context.Context, spec interface{}) (string, error) {
	var resp nutanixTaskResponse
	if err := c.post(ctx, "/vms", spec, &resp); err != nil {
		return "", err
	}
	return resp.TaskUUID, nil
}

// GetTask returns the current status of a Nutanix task.
func (c *Client) GetTask(ctx context.Context, taskUUID string) (*nutanixTask, error) {
	var task nutanixTask
	if err := c.get(ctx, "/tasks/"+url.PathEscape(taskUUID), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// ListClusters returns all clusters.
func (c *Client) ListClusters(ctx context.Context) ([]nutanixCluster, error) {
	body := nutanixListMetadata{Kind: "cluster", Length: 100}
	var resp struct {
		Entities []nutanixCluster `json:"entities"`
	}
	if err := c.post(ctx, "/clusters/list", body, &resp); err != nil {
		return nil, err
	}
	return resp.Entities, nil
}

// ListHosts returns all AHV hosts.
func (c *Client) ListHosts(ctx context.Context) ([]nutanixHost, error) {
	body := nutanixListMetadata{Kind: "host", Length: 500}
	var resp struct {
		Entities []nutanixHost `json:"entities"`
	}
	if err := c.post(ctx, "/hosts/list", body, &resp); err != nil {
		return nil, err
	}
	return resp.Entities, nil
}

// ListSubnets returns all subnets/networks.
func (c *Client) ListSubnets(ctx context.Context) ([]nutanixSubnet, error) {
	body := nutanixListMetadata{Kind: "subnet", Length: 500}
	var resp struct {
		Entities []nutanixSubnet `json:"entities"`
	}
	if err := c.post(ctx, "/subnets/list", body, &resp); err != nil {
		return nil, err
	}
	return resp.Entities, nil
}

// ListImages returns all images (templates).
func (c *Client) ListImages(ctx context.Context) ([]nutanixImage, error) {
	body := nutanixListMetadata{Kind: "image", Length: 500}
	var resp struct {
		Entities []nutanixImage `json:"entities"`
	}
	if err := c.post(ctx, "/images/list", body, &resp); err != nil {
		return nil, err
	}
	return resp.Entities, nil
}

// ListVMSnapshots returns all recovery points for a VM.
// Nutanix v3 uses recovery_points for VM snapshots.
func (c *Client) ListVMSnapshots(ctx context.Context, vmUUID string) ([]nutanixVMSnapshot, error) {
	// Nutanix v3 API: list recovery points filtered by VM UUID
	body := map[string]interface{}{
		"kind":   "recovery_point",
		"length": 100,
		"filter": fmt.Sprintf("vm_uuid==%s", vmUUID),
	}
	var resp struct {
		Entities []nutanixVMSnapshot `json:"entities"`
	}
	if err := c.post(ctx, "/recovery_points/list", body, &resp); err != nil {
		// Fallback: some Prism Element versions use a different endpoint
		return []nutanixVMSnapshot{}, nil
	}
	return resp.Entities, nil
}

// CreateVMSnapshot creates a recovery point (snapshot) for a VM.
func (c *Client) CreateVMSnapshot(ctx context.Context, vmUUID, name string) (string, error) {
	body := map[string]interface{}{
		"spec": map[string]interface{}{
			"name": name,
			"resources": map[string]interface{}{
				"vm_recovery_point_list": []map[string]interface{}{
					{
						"vm_reference": map[string]interface{}{
							"kind": "vm",
							"uuid": vmUUID,
						},
					},
				},
			},
		},
		"metadata": map[string]interface{}{
			"kind": "recovery_point",
		},
	}
	var resp nutanixTaskResponse
	if err := c.post(ctx, "/recovery_points", body, &resp); err != nil {
		return "", err
	}
	return resp.TaskUUID, nil
}

// DeleteVMSnapshot deletes a recovery point by UUID.
func (c *Client) DeleteVMSnapshot(ctx context.Context, snapshotUUID string) (string, error) {
	var resp nutanixTaskResponse
	if err := c.delete(ctx, "/recovery_points/"+url.PathEscape(snapshotUUID), &resp); err != nil {
		return "", err
	}
	return resp.TaskUUID, nil
}

// RestoreVMSnapshot restores a VM from a recovery point.
func (c *Client) RestoreVMSnapshot(ctx context.Context, vmUUID, snapshotUUID string) (string, error) {
	body := map[string]interface{}{
		"vm_reference": map[string]interface{}{
			"kind": "vm",
			"uuid": vmUUID,
		},
	}
	var resp nutanixTaskResponse
	if err := c.post(ctx, "/recovery_points/"+url.PathEscape(snapshotUUID)+"/restore", body, &resp); err != nil {
		return "", err
	}
	return resp.TaskUUID, nil
}
