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
	"fmt"
	"net/url"
	"time"

	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/pkg/logger"
)

// timeNow is a package-level variable so tests can override it.
var timeNow = time.Now

// consoleDefaultTTL returns opts.TTL if set, otherwise 5 minutes.
func consoleDefaultTTL(opts port.ConsoleOptions) time.Duration {
	if opts.TTL > 0 {
		return opts.TTL
	}
	return 5 * time.Minute
}

// Provider implements port.Provider and port.ConsoleProvider for Proxmox VE.
type Provider struct {
	provider.BaseProvider
	cfg    config.ProxmoxConfig
	log    logger.Logger
	client *Client // nil until Connect is called
}

// NewProvider creates a new Proxmox provider instance.
// The provider is not connected until Connect is called.
func NewProvider(cfg config.ProxmoxConfig, log logger.Logger) *Provider {
	return &Provider{cfg: cfg, log: log}
}

// ── Meta ─────────────────────────────────────────────────────────────────────

func (p *Provider) Type() model.ProviderType { return model.ProviderProxmox }
func (p *Provider) Name() string             { return "Proxmox VE" }

// Capabilities declares the feature set supported by this provider.
func (p *Provider) Capabilities() port.ProviderCapabilities {
	return port.ProviderCapabilities{
		Console:          true,  // noVNC via vncproxy
		LinkedClones:     false, // Proxmox uses full clones by default
		MemorySnapshots:  true,  // vmstate=1 in snapshot
		QuiesceSnapshots: false, // requires QEMU guest agent
		LiveMigration:    false, // requires shared storage
		GuestMetrics:     true,  // RRD data via /rrddata
		TemplateClone:    true,  // clone from template VMID
	}
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

// Connect initialises the Proxmox REST client and verifies connectivity.
// Credentials.Token must be in the form "<user>@<realm>!<tokenname>=<secret>",
// e.g. "root@pam!vmorbit=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
// Alternatively, set Token to "<user>@<realm>!<tokenname>" and Password to the
// UUID secret — the provider will split them automatically.
func (p *Provider) Connect(ctx context.Context, creds port.Credentials) error {
	p.log.Info("proxmox: connecting",
		logger.String("host", creds.Host),
		logger.Bool("tls_verify", creds.TLSVerify),
	)

	tokenID, tokenValue, err := resolveToken(creds)
	if err != nil {
		return provider.Wrap("proxmox", "Connect", provider.CodeAuthFailed,
			"invalid API token format", err)
	}

	timeout := p.cfg.DefaultTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var c *Client
	connectErr := withRetry(ctx, retryConfig{
		maxAttempts: 3,
		baseDelay:   2 * time.Second,
		maxDelay:    10 * time.Second,
		shouldRetry: isRetryableConnectError,
	}, func(attempt int) error {
		if attempt > 1 {
			p.log.Warn("proxmox: retrying connection",
				logger.String("host", creds.Host),
				logger.Int("attempt", attempt),
			)
		}
		c = newClient(creds.Host, creds.Port, tokenID, tokenValue, creds.TLSVerify, timeout)
		_, pingErr := c.GetVersion(ctx)
		return pingErr
	})
	if connectErr != nil {
		if isProxmoxAuthError(connectErr) {
			return provider.Wrap("proxmox", "Connect", provider.CodeAuthFailed,
				fmt.Sprintf("authentication failed for %s", creds.Host), connectErr)
		}
		return provider.Wrap("proxmox", "Connect", provider.CodeNotConnected,
			fmt.Sprintf("failed to reach Proxmox at %s", creds.Host), connectErr)
	}

	p.client = c
	p.StoreCredentials(creds)
	p.SetConnected(true)
	p.log.Info("proxmox: connected", logger.String("host", creds.Host))
	return nil
}

// Disconnect releases the HTTP client and marks the provider as disconnected.
func (p *Provider) Disconnect(_ context.Context) error {
	p.client = nil
	p.SetConnected(false)
	p.log.Info("proxmox: disconnected")
	return nil
}

// Ping verifies the Proxmox API is reachable and the token is still valid.
func (p *Provider) Ping(ctx context.Context) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	if _, err := p.client.GetVersion(ctx); err != nil {
		p.SetConnected(false)
		return provider.Wrap("proxmox", "Ping", provider.CodeNotConnected,
			"version check failed", err)
	}
	return nil
}

// ── VM Operations ────────────────────────────────────────────────────────────

// ListVMs returns all QEMU VMs across all cluster nodes using the efficient
// cluster/resources endpoint (single API call for the whole cluster).
func (p *Provider) ListVMs(ctx context.Context) ([]port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	resources, err := p.client.ListClusterResources(ctx, "vm")
	if err != nil {
		return nil, provider.Wrap("proxmox", "ListVMs", provider.CodeInternal,
			"failed to list cluster resources", err)
	}

	infos := make([]port.VMInfo, 0, len(resources))
	for i := range resources {
		r := &resources[i]
		if r.Type != "qemu" {
			continue // skip LXC containers and other resource types
		}
		info := mapResourceToVMInfo(r)
		infos = append(infos, info)
	}

	p.log.Info("proxmox: listed VMs", logger.Int("count", len(infos)))
	return infos, nil
}

// GetVM retrieves full details for a single VM by its provider ID ("<node>/<vmid>").
func (p *Provider) GetVM(ctx context.Context, providerVMID string) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return nil, provider.Wrap("proxmox", "GetVM", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	status, err := p.client.GetVMStatus(ctx, node, vmid)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return nil, provider.New("proxmox", "GetVM", provider.CodeNotFound,
				fmt.Sprintf("VM %q not found", providerVMID))
		}
		return nil, provider.Wrap("proxmox", "GetVM", provider.CodeInternal,
			"failed to get VM status", err)
	}

	cfg, err := p.client.GetVMConfig(ctx, node, vmid)
	if err != nil {
		// Config fetch is best-effort; proceed with status-only data.
		p.log.Warn("proxmox: failed to fetch VM config (using status only)",
			logger.String("vm_id", providerVMID), logger.Error(err))
		cfg = nil
	}

	info := port.VMInfo{
		ProviderVMID: providerVMID,
		Extra:        map[string]interface{}{"node": node, "vmid": vmid},
	}
	enrichVMInfo(&info, status, cfg)
	return &info, nil
}

// CreateVM creates a new QEMU VM on the first available node and waits for
// the Proxmox task to complete before returning.
func (p *Provider) CreateVM(ctx context.Context, spec port.VMCreateSpec) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	node, err := p.pickNode(ctx)
	if err != nil {
		return nil, err
	}

	// Allocate the next available VMID.
	vmid, err := p.nextVMID(ctx)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("vmid", fmt.Sprintf("%d", vmid))
	params.Set("name", spec.Name)
	params.Set("memory", fmt.Sprintf("%d", spec.MemoryMB))
	params.Set("cores", fmt.Sprintf("%d", spec.CPUCount))
	params.Set("sockets", "1")
	if spec.NetworkName != "" {
		params.Set("net0", fmt.Sprintf("virtio,bridge=%s", spec.NetworkName))
	}
	if spec.DiskGB > 0 {
		storage := spec.DataStore
		if storage == "" {
			storage = "local-lvm"
		}
		params.Set("scsi0", fmt.Sprintf("%s:%d", storage, spec.DiskGB))
		params.Set("scsihw", "virtio-scsi-pci")
	}
	if spec.GuestOS != "" {
		params.Set("ostype", spec.GuestOS)
	}

	var upid string
	err = withOperationRetry(ctx, func(_ int) error {
		var e error
		upid, e = p.client.CreateVM(ctx, node, params)
		return e
	})
	if err != nil {
		return nil, provider.Wrap("proxmox", "CreateVM", provider.CodeInternal,
			"failed to create VM", err)
	}

	if err := waitForTask(ctx, p.client, upid, node, defaultTaskPollConfig()); err != nil {
		return nil, provider.Wrap("proxmox", "CreateVM", provider.CodeInternal,
			"VM creation task failed", err)
	}

	pid := buildProviderVMID(node, vmid)
	p.log.Info("proxmox: VM created", logger.String("vm_id", pid))
	return p.GetVM(ctx, pid)
}

// DeleteVM destroys a QEMU VM and removes it from the cluster configuration.
func (p *Provider) DeleteVM(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return provider.Wrap("proxmox", "DeleteVM", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	// Stop the VM first if it is running.
	if stopErr := p.stopIfRunning(ctx, node, vmid); stopErr != nil {
		p.log.Warn("proxmox: could not stop VM before delete (proceeding anyway)",
			logger.String("vm_id", providerVMID), logger.Error(stopErr))
	}

	upid, err := p.client.DeleteVM(ctx, node, vmid, true)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return provider.New("proxmox", "DeleteVM", provider.CodeNotFound,
				fmt.Sprintf("VM %q not found", providerVMID))
		}
		return provider.Wrap("proxmox", "DeleteVM", provider.CodeInternal,
			"failed to delete VM", err)
	}

	if err := waitForTask(ctx, p.client, upid, node, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("proxmox", "DeleteVM", provider.CodeInternal,
			"VM deletion task failed", err)
	}

	p.log.Info("proxmox: VM deleted", logger.String("vm_id", providerVMID))
	return nil
}

// CloneVM creates a full clone of an existing VM or template.
func (p *Provider) CloneVM(ctx context.Context, providerVMID string, spec port.VMCloneSpec) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return nil, provider.Wrap("proxmox", "CloneVM", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	newVMID, err := p.nextVMID(ctx)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("newid", fmt.Sprintf("%d", newVMID))
	params.Set("name", spec.Name)
	params.Set("full", "1") // always full clone
	if spec.DataStore != "" {
		params.Set("storage", spec.DataStore)
	}

	var upid string
	err = withOperationRetry(ctx, func(_ int) error {
		var e error
		upid, e = p.client.CloneVM(ctx, node, vmid, params)
		return e
	})
	if err != nil {
		return nil, provider.Wrap("proxmox", "CloneVM", provider.CodeInternal,
			"failed to clone VM", err)
	}

	// Clone tasks can take a while — use a longer timeout.
	pollCfg := taskPollConfig{interval: 3 * time.Second, timeout: 15 * time.Minute}
	if err := waitForTask(ctx, p.client, upid, node, pollCfg); err != nil {
		return nil, provider.Wrap("proxmox", "CloneVM", provider.CodeInternal,
			"VM clone task failed", err)
	}

	pid := buildProviderVMID(node, newVMID)
	p.log.Info("proxmox: VM cloned",
		logger.String("source_vm_id", providerVMID),
		logger.String("new_vm_id", pid),
	)
	return p.GetVM(ctx, pid)
}

// ── Power Operations ─────────────────────────────────────────────────────────

// PowerOn starts a stopped or suspended VM and waits for the task to complete.
func (p *Provider) PowerOn(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "start", "PowerOn")
}

// PowerOff hard-stops a running VM (equivalent to pulling the power cord).
func (p *Provider) PowerOff(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "stop", "PowerOff")
}

// Reboot issues a graceful ACPI reboot to the guest OS.
func (p *Provider) Reboot(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "reboot", "Reboot")
}

// Suspend suspends the VM to RAM (pause).
func (p *Provider) Suspend(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "suspend", "Suspend")
}

// Reset performs a hard reset (power cycle) without saving state.
func (p *Provider) Reset(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "reset", "Reset")
}

// powerAction is the shared implementation for all power state transitions.
// action must be one of: start, stop, reboot, suspend, reset, shutdown.
func (p *Provider) powerAction(ctx context.Context, providerVMID, action, opName string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return provider.Wrap("proxmox", opName, provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	var upid string
	err = withOperationRetry(ctx, func(_ int) error {
		var e error
		upid, e = p.client.VMPowerAction(ctx, node, vmid, action)
		return e
	})
	if err != nil {
		if ae, ok := err.(*apiError); ok {
			if ae.isNotFound() {
				return provider.New("proxmox", opName, provider.CodeNotFound,
					fmt.Sprintf("VM %q not found", providerVMID))
			}
			if ae.StatusCode == 409 {
				return provider.New("proxmox", opName, provider.CodeInvalidState,
					fmt.Sprintf("VM %q is in an invalid state for %s", providerVMID, action))
			}
		}
		return provider.Wrap("proxmox", opName, provider.CodeInternal,
			fmt.Sprintf("power action %q failed", action), err)
	}

	if err := waitForTask(ctx, p.client, upid, node, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("proxmox", opName, provider.CodeInternal,
			fmt.Sprintf("power action %q task failed", action), err)
	}

	p.log.Info("proxmox: power action completed",
		logger.String("vm_id", providerVMID),
		logger.String("action", action),
	)
	return nil
}

// ── Metrics ──────────────────────────────────────────────────────────────────

// GetVMMetrics fetches the most recent RRD performance data for a VM.
func (p *Provider) GetVMMetrics(ctx context.Context, providerVMID string) (*port.VMMetrics, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return nil, provider.Wrap("proxmox", "GetVMMetrics", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	data, err := p.client.GetVMRRDData(ctx, node, vmid, "hour")
	if err != nil {
		return nil, provider.Wrap("proxmox", "GetVMMetrics", provider.CodeInternal,
			"failed to fetch RRD data", err)
	}

	// Fetch maxmem from status for accurate memory percentage calculation.
	var maxMem int64
	if status, sErr := p.client.GetVMStatus(ctx, node, vmid); sErr == nil {
		maxMem = status.MaxMem
	}

	return mapRRDMetrics(data, maxMem), nil
}

// ── Snapshots ────────────────────────────────────────────────────────────────

// ListSnapshots returns all snapshots for a VM, excluding the special
// "current" pseudo-snapshot that Proxmox always includes.
func (p *Provider) ListSnapshots(ctx context.Context, providerVMID string) ([]port.SnapshotInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return nil, provider.Wrap("proxmox", "ListSnapshots", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	snaps, err := p.client.ListSnapshots(ctx, node, vmid)
	if err != nil {
		return nil, provider.Wrap("proxmox", "ListSnapshots", provider.CodeInternal,
			"failed to list snapshots", err)
	}

	// Find the current snapshot name (the one with no parent that is "current").
	currentName := ""
	for i := range snaps {
		if snaps[i].Name == "current" {
			currentName = snaps[i].Parent
			break
		}
	}

	infos := make([]port.SnapshotInfo, 0, len(snaps))
	for i := range snaps {
		if snaps[i].Name == "current" {
			continue // skip the pseudo-snapshot
		}
		infos = append(infos, mapSnapshot(&snaps[i], currentName))
	}
	return infos, nil
}

// CreateSnapshot takes a named snapshot of the VM with optional memory state.
func (p *Provider) CreateSnapshot(ctx context.Context, providerVMID string, spec port.SnapshotSpec) (*port.SnapshotInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return nil, provider.Wrap("proxmox", "CreateSnapshot", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	params := url.Values{}
	params.Set("snapname", spec.Name)
	if spec.Description != "" {
		params.Set("description", spec.Description)
	}
	if spec.Memory {
		params.Set("vmstate", "1")
	}

	var upid string
	err = withOperationRetry(ctx, func(_ int) error {
		var e error
		upid, e = p.client.CreateSnapshot(ctx, node, vmid, params)
		return e
	})
	if err != nil {
		return nil, provider.Wrap("proxmox", "CreateSnapshot", provider.CodeInternal,
			"failed to create snapshot", err)
	}

	if err := waitForTask(ctx, p.client, upid, node, defaultTaskPollConfig()); err != nil {
		return nil, provider.Wrap("proxmox", "CreateSnapshot", provider.CodeInternal,
			"snapshot task failed", err)
	}

	info := &port.SnapshotInfo{
		ProviderID:  spec.Name,
		Name:        spec.Name,
		Description: spec.Description,
		IsCurrent:   true,
		CreatedAt:   timeNow(),
	}
	p.log.Info("proxmox: snapshot created",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot", spec.Name),
	)
	return info, nil
}

// DeleteSnapshot removes a snapshot by name.
func (p *Provider) DeleteSnapshot(ctx context.Context, providerVMID, snapshotID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return provider.Wrap("proxmox", "DeleteSnapshot", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	upid, err := p.client.DeleteSnapshot(ctx, node, vmid, snapshotID)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return provider.New("proxmox", "DeleteSnapshot", provider.CodeNotFound,
				fmt.Sprintf("snapshot %q not found", snapshotID))
		}
		return provider.Wrap("proxmox", "DeleteSnapshot", provider.CodeInternal,
			"failed to delete snapshot", err)
	}

	if err := waitForTask(ctx, p.client, upid, node, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("proxmox", "DeleteSnapshot", provider.CodeInternal,
			"snapshot deletion task failed", err)
	}

	p.log.Info("proxmox: snapshot deleted",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot", snapshotID),
	)
	return nil
}

// RevertSnapshot rolls the VM back to the named snapshot.
func (p *Provider) RevertSnapshot(ctx context.Context, providerVMID, snapshotID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return provider.Wrap("proxmox", "RevertSnapshot", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	upid, err := p.client.RollbackSnapshot(ctx, node, vmid, snapshotID)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return provider.New("proxmox", "RevertSnapshot", provider.CodeNotFound,
				fmt.Sprintf("snapshot %q not found", snapshotID))
		}
		return provider.Wrap("proxmox", "RevertSnapshot", provider.CodeInternal,
			"failed to revert snapshot", err)
	}

	if err := waitForTask(ctx, p.client, upid, node, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("proxmox", "RevertSnapshot", provider.CodeInternal,
			"snapshot revert task failed", err)
	}

	p.log.Info("proxmox: snapshot reverted",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot", snapshotID),
	)
	return nil
}

// ── Storage ──────────────────────────────────────────────────────────────────

// ListDataStores returns all active storage pools across all cluster nodes.
// Duplicate storage names (shared storage appearing on multiple nodes) are
// deduplicated — only the first occurrence is returned.
func (p *Provider) ListDataStores(ctx context.Context) ([]port.DataStoreInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	nodes, err := p.client.ListNodes(ctx)
	if err != nil {
		return nil, provider.Wrap("proxmox", "ListDataStores", provider.CodeInternal,
			"failed to list nodes", err)
	}

	seen := make(map[string]struct{})
	var infos []port.DataStoreInfo

	for _, node := range nodes {
		if node.Status != "online" {
			continue
		}
		stores, err := p.client.ListStorage(ctx, node.Node)
		if err != nil {
			p.log.Warn("proxmox: failed to list storage on node",
				logger.String("node", node.Node), logger.Error(err))
			continue
		}
		for i := range stores {
			s := &stores[i]
			if _, dup := seen[s.Storage]; dup {
				continue
			}
			seen[s.Storage] = struct{}{}
			infos = append(infos, mapStorage(s, node.Node))
		}
	}
	return infos, nil
}

// GetDataStore retrieves a single storage pool by its provider ID ("<node>/<storage>").
func (p *Provider) GetDataStore(ctx context.Context, id string) (*port.DataStoreInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	stores, err := p.ListDataStores(ctx)
	if err != nil {
		return nil, err
	}
	for i := range stores {
		if stores[i].ProviderID == id || stores[i].Name == id {
			return &stores[i], nil
		}
	}
	return nil, provider.New("proxmox", "GetDataStore", provider.CodeNotFound,
		fmt.Sprintf("datastore %q not found", id))
}

// ── Networks ─────────────────────────────────────────────────────────────────

// ListNetworks returns all bridge-type network interfaces across all nodes.
// Only bridge interfaces are relevant for VM networking in Proxmox.
func (p *Provider) ListNetworks(ctx context.Context) ([]port.NetworkInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	nodes, err := p.client.ListNodes(ctx)
	if err != nil {
		return nil, provider.Wrap("proxmox", "ListNetworks", provider.CodeInternal,
			"failed to list nodes", err)
	}

	seen := make(map[string]struct{})
	var infos []port.NetworkInfo

	for _, node := range nodes {
		if node.Status != "online" {
			continue
		}
		nets, err := p.client.ListNetworks(ctx, node.Node)
		if err != nil {
			p.log.Warn("proxmox: failed to list networks on node",
				logger.String("node", node.Node), logger.Error(err))
			continue
		}
		for i := range nets {
			n := &nets[i]
			if n.Type != "bridge" {
				continue // only bridges are usable for VMs
			}
			if _, dup := seen[n.Iface]; dup {
				continue
			}
			seen[n.Iface] = struct{}{}
			infos = append(infos, mapNetwork(n, node.Node))
		}
	}
	return infos, nil
}

// GetNetwork retrieves a single network by its provider ID ("<node>/<iface>") or name.
func (p *Provider) GetNetwork(ctx context.Context, id string) (*port.NetworkInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	nets, err := p.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	for i := range nets {
		if nets[i].ProviderID == id || nets[i].Name == id {
			return &nets[i], nil
		}
	}
	return nil, provider.New("proxmox", "GetNetwork", provider.CodeNotFound,
		fmt.Sprintf("network %q not found", id))
}

// ── Inventory Sync ───────────────────────────────────────────────────────────

// SyncInventory fetches the full live inventory from Proxmox using concurrent
// goroutines for VMs, storage, and networks, then returns a normalised snapshot
// for the service layer to reconcile against the database.
func (p *Provider) SyncInventory(ctx context.Context) (*port.InventorySnapshot, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	type result[T any] struct {
		data []T
		err  error
	}
	vmCh := make(chan result[port.VMInfo], 1)
	dsCh := make(chan result[port.DataStoreInfo], 1)
	netCh := make(chan result[port.NetworkInfo], 1)

	go func() {
		vms, err := p.ListVMs(ctx)
		vmCh <- result[port.VMInfo]{data: vms, err: err}
	}()
	go func() {
		ds, err := p.ListDataStores(ctx)
		dsCh <- result[port.DataStoreInfo]{data: ds, err: err}
	}()
	go func() {
		nets, err := p.ListNetworks(ctx)
		netCh <- result[port.NetworkInfo]{data: nets, err: err}
	}()

	vmRes := <-vmCh
	dsRes := <-dsCh
	netRes := <-netCh

	if vmRes.err != nil {
		return nil, fmt.Errorf("proxmox SyncInventory ListVMs: %w", vmRes.err)
	}
	if dsRes.err != nil {
		return nil, fmt.Errorf("proxmox SyncInventory ListDataStores: %w", dsRes.err)
	}
	if netRes.err != nil {
		return nil, fmt.Errorf("proxmox SyncInventory ListNetworks: %w", netRes.err)
	}

	snap := &port.InventorySnapshot{
		VMs:        vmRes.data,
		DataStores: dsRes.data,
		Networks:   netRes.data,
		SyncedAt:   timeNow(),
	}

	// Populate Hosts from Proxmox nodes
	nodes, err := p.client.ListNodes(ctx)
	if err == nil {
		for _, node := range nodes {
			status := "disconnected"
			if node.Status == "online" {
				status = "connected"
			}
			totalMemMB := int(node.MaxMem / 1024 / 1024)
			usedMemMB := int(node.Mem / 1024 / 1024)
			snap.Hosts = append(snap.Hosts, port.HostInfo{
				ProviderID:    node.Node,
				Name:          node.Node,
				Status:        status,
				CPUCores:      node.MaxCPU,
				TotalMemoryMB: totalMemMB,
				UsedMemoryMB:  usedMemMB,
				UptimeSeconds: node.Uptime,
				Extra: map[string]interface{}{
					"cpu_usage": node.CPU,
				},
			})
		}
	}

	p.log.Info("proxmox: inventory sync complete",
		logger.Int("vms", len(snap.VMs)),
		logger.Int("datastores", len(snap.DataStores)),
		logger.Int("networks", len(snap.Networks)),
	)
	return snap, nil
}

// ── Console Session ──────────────────────────────────────────────────────────

// GetConsoleSession creates a noVNC console session for the given VM.
// Returns an HTTPS URL that opens the Proxmox built-in noVNC console directly
// in a browser — no additional client software required.
func (p *Provider) GetConsoleSession(ctx context.Context, providerVMID string, opts port.ConsoleOptions) (*port.ConsoleSession, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	node, vmid, err := parseProviderVMID(providerVMID)
	if err != nil {
		return nil, provider.Wrap("proxmox", "GetConsoleSession", provider.CodeNotFound,
			"invalid provider VM ID", err)
	}

	proxy, err := p.client.VNCProxy(ctx, node, vmid, true /* websocket */)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return nil, provider.New("proxmox", "GetConsoleSession", provider.CodeNotFound,
				fmt.Sprintf("VM %q not found", providerVMID))
		}
		return nil, provider.Wrap("proxmox", "GetConsoleSession", provider.CodeInternal,
			"failed to create VNC proxy session", err)
	}

	creds := p.Credentials()
	host := creds.Host
	apiPort := creds.Port
	if apiPort == 0 {
		apiPort = 8006
	}

	ttl := consoleDefaultTTL(opts)

	// Proxmox built-in noVNC console URL — opens directly in any browser.
	// The ticket is passed as a query parameter; Proxmox validates it server-side.
	// Format: https://<host>:<port>/?console=kvm&novnc=1&vmid=<vmid>&node=<node>&ticket=<ticket>
	consoleURL := fmt.Sprintf(
		"https://%s:%d/?console=kvm&novnc=1&vmid=%d&node=%s&ticket=%s",
		host, apiPort, vmid, url.QueryEscape(node), url.QueryEscape(proxy.Ticket),
	)

	// Raw WebSocket URL for embedding (kept in Extra for future use).
	wssPort := int(proxy.Port)
	if wssPort == 0 {
		wssPort = apiPort
	}
	wssURL := fmt.Sprintf("wss://%s:%d/?ticket=%s", host, wssPort, url.QueryEscape(proxy.Ticket))

	session := &port.ConsoleSession{
		Type:      port.ConsoleTypeNoVNC,
		URL:       consoleURL,
		Ticket:    proxy.Ticket,
		Host:      host,
		Port:      apiPort,
		ExpiresAt: timeNow().Add(ttl),
		Extra: map[string]interface{}{
			"provider": "proxmox",
			"node":     node,
			"vmid":     vmid,
			"vnc_port": int(proxy.Port),
			"ticket":   proxy.Ticket,
			"wss_url":  wssURL,
			"cert":     proxy.Cert,
			"user":     proxy.User,
		},
	}

	p.log.Info("proxmox: console session created",
		logger.String("vm_id", providerVMID),
		logger.String("url", consoleURL),
	)
	return session, nil
}

// ── Templates ────────────────────────────────────────────────────────────────

// ListTemplates returns all QEMU VMs that are marked as templates in Proxmox.
func (p *Provider) ListTemplates(ctx context.Context) ([]port.TemplateInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	resources, err := p.client.ListClusterResources(ctx, "vm")
	if err != nil {
		return nil, provider.Wrap("proxmox", "ListTemplates", provider.CodeInternal,
			"failed to list cluster resources", err)
	}

	var infos []port.TemplateInfo
	for i := range resources {
		r := &resources[i]
		if r.Type != "qemu" || r.Template != 1 {
			continue // only QEMU templates
		}
		info := port.TemplateInfo{
			ProviderID: buildProviderVMID(r.Node, r.VMID),
			Name:       r.Name,
			GuestOS:    r.OSType,
			CPUCount:   int(r.MaxCPU),
			MemoryMB:   int(r.MaxMem / 1024 / 1024),
			DiskGB:     int(r.MaxDisk / 1024 / 1024 / 1024),
			Extra: map[string]interface{}{
				"node": r.Node,
				"vmid": r.VMID,
			},
		}
		infos = append(infos, info)
	}

	p.log.Info("proxmox: listed templates", logger.Int("count", len(infos)))
	return infos, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// pickNode returns the name of the first online cluster node.
// In a single-node setup this is always the one node.
func (p *Provider) pickNode(ctx context.Context) (string, error) {
	nodes, err := p.client.ListNodes(ctx)
	if err != nil {
		return "", provider.Wrap("proxmox", "pickNode", provider.CodeInternal,
			"failed to list nodes", err)
	}
	for _, n := range nodes {
		if n.Status == "online" {
			return n.Node, nil
		}
	}
	return "", provider.New("proxmox", "pickNode", provider.CodeInternal,
		"no online nodes available")
}

// nextVMID asks Proxmox for the next available VMID.
// Proxmox may return the VMID as either a JSON number or a quoted string
// depending on the PVE version, so flexInt handles both forms.
func (p *Provider) nextVMID(ctx context.Context) (int, error) {
	var vmid flexInt
	if err := p.client.get(ctx, "/cluster/nextid", &vmid); err != nil {
		return 0, provider.Wrap("proxmox", "nextVMID", provider.CodeInternal,
			"failed to allocate VMID", err)
	}
	return int(vmid), nil
}

// stopIfRunning stops a VM if it is currently running.
// It waits for the stop task to complete before returning.
func (p *Provider) stopIfRunning(ctx context.Context, node string, vmid int) error {
	status, err := p.client.GetVMStatus(ctx, node, vmid)
	if err != nil {
		return err
	}
	if status.Status != "running" {
		return nil
	}

	upid, err := p.client.VMPowerAction(ctx, node, vmid, "stop")
	if err != nil {
		return err
	}
	return waitForTask(ctx, p.client, upid, node, defaultTaskPollConfig())
}

// resolveToken extracts the tokenID and tokenValue from the credentials.
//
// Supported formats:
//  1. creds.Token = "user@realm!name=uuid-secret"  (full token string)
//  2. creds.Token = "user@realm!name", creds.Password = "uuid-secret"
//  3. creds.Username = "user@realm!name", creds.Password = "uuid-secret"
func resolveToken(creds port.Credentials) (tokenID, tokenValue string, err error) {
	// Format 1: full token in Token field.
	if idx := indexByte(creds.Token, '='); idx > 0 && idx < len(creds.Token)-1 {
		return creds.Token[:idx], creds.Token[idx+1:], nil
	}

	// Format 2: token ID in Token field, secret in Password.
	if creds.Token != "" && creds.Password != "" {
		return creds.Token, creds.Password, nil
	}

	// Format 3: token ID in Username field, secret in Password.
	if creds.Username != "" && creds.Password != "" {
		return creds.Username, creds.Password, nil
	}

	return "", "", fmt.Errorf(
		"proxmox: API token not configured — set Token to 'user@realm!name=secret' " +
			"or set Token/Username to the token ID and Password to the secret")
}

// indexByte returns the index of the first occurrence of b in s, or -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
