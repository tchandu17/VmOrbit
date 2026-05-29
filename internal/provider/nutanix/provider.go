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
	"context"
	"fmt"
	"time"

	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/pkg/logger"
)

// timeNow is a package-level variable so tests can override it.
var timeNow = time.Now

// Provider implements port.Provider for Nutanix AHV via Prism Element/Central.
type Provider struct {
	provider.BaseProvider
	cfg    config.NutanixConfig
	log    logger.Logger
	client *Client // nil until Connect is called
}

// NewProvider creates a new Nutanix provider instance.
// The provider is not connected until Connect is called.
func NewProvider(cfg config.NutanixConfig, log logger.Logger) *Provider {
	return &Provider{cfg: cfg, log: log}
}

// ── Meta ─────────────────────────────────────────────────────────────────────

func (p *Provider) Type() model.ProviderType { return model.ProviderNutanix }
func (p *Provider) Name() string             { return "Nutanix AHV" }

// Capabilities declares the feature set supported by this provider.
func (p *Provider) Capabilities() port.ProviderCapabilities {
	return port.ProviderCapabilities{
		Console:          false, // Nutanix console requires VNC via separate flow
		LinkedClones:     false, // Nutanix uses full clones
		MemorySnapshots:  false, // Nutanix recovery points are crash-consistent
		QuiesceSnapshots: false,
		LiveMigration:    false,
		GuestMetrics:     false, // Nutanix metrics require Prism Pro / separate API
		TemplateClone:    true,  // Clone from image
	}
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

// Connect initialises the Nutanix Prism REST client and verifies connectivity.
// Credentials.Username and Credentials.Password are used for Basic Auth.
func (p *Provider) Connect(ctx context.Context, creds port.Credentials) error {
	p.log.Info("nutanix: connecting",
		logger.String("host", creds.Host),
		logger.Bool("tls_verify", creds.TLSVerify),
	)

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
			p.log.Warn("nutanix: retrying connection",
				logger.String("host", creds.Host),
				logger.Int("attempt", attempt),
			)
		}
		c = newClient(creds.Host, creds.Port, creds.Username, creds.Password, creds.TLSVerify, timeout)
		return c.Ping(ctx)
	})
	if connectErr != nil {
		if isNutanixAuthError(connectErr) {
			return provider.Wrap("nutanix", "Connect", provider.CodeAuthFailed,
				fmt.Sprintf("authentication failed for %s", creds.Host), connectErr)
		}
		return provider.Wrap("nutanix", "Connect", provider.CodeNotConnected,
			fmt.Sprintf("failed to reach Nutanix Prism at %s", creds.Host), connectErr)
	}

	p.client = c
	p.StoreCredentials(creds)
	p.SetConnected(true)
	p.log.Info("nutanix: connected", logger.String("host", creds.Host))
	return nil
}

// Disconnect releases the HTTP client and marks the provider as disconnected.
func (p *Provider) Disconnect(_ context.Context) error {
	p.client = nil
	p.SetConnected(false)
	p.log.Info("nutanix: disconnected")
	return nil
}

// Ping verifies the Nutanix Prism API is reachable and credentials are valid.
func (p *Provider) Ping(ctx context.Context) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	if err := p.client.Ping(ctx); err != nil {
		p.SetConnected(false)
		return provider.Wrap("nutanix", "Ping", provider.CodeNotConnected,
			"cluster list check failed", err)
	}
	return nil
}

// ── VM Operations ────────────────────────────────────────────────────────────

// ListVMs returns all VMs from Nutanix Prism.
func (p *Provider) ListVMs(ctx context.Context) ([]port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	vms, err := p.client.ListVMs(ctx)
	if err != nil {
		return nil, provider.Wrap("nutanix", "ListVMs", provider.CodeInternal,
			"failed to list VMs", err)
	}

	// Build cluster and host maps for enrichment.
	clusterMap, hostMap := p.buildTopologyMaps(ctx)

	infos := make([]port.VMInfo, 0, len(vms))
	for i := range vms {
		infos = append(infos, mapVMToInfo(&vms[i], clusterMap, hostMap))
	}

	p.log.Info("nutanix: listed VMs", logger.Int("count", len(infos)))
	return infos, nil
}

// GetVM retrieves full details for a single VM by its UUID.
func (p *Provider) GetVM(ctx context.Context, providerVMID string) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	vm, err := p.client.GetVM(ctx, providerVMID)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return nil, provider.New("nutanix", "GetVM", provider.CodeNotFound,
				fmt.Sprintf("VM %q not found", providerVMID))
		}
		return nil, provider.Wrap("nutanix", "GetVM", provider.CodeInternal,
			"failed to get VM", err)
	}

	clusterMap, hostMap := p.buildTopologyMaps(ctx)
	info := mapVMToInfo(vm, clusterMap, hostMap)
	return &info, nil
}

// CreateVM creates a new VM on Nutanix AHV and waits for the task to complete.
func (p *Provider) CreateVM(ctx context.Context, spec port.VMCreateSpec) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	// Pick the first available cluster UUID.
	clusterUUID, err := p.pickCluster(ctx)
	if err != nil {
		return nil, err
	}

	body := buildVMCreateSpec(spec, clusterUUID)

	var taskUUID string
	err = withOperationRetry(ctx, func(_ int) error {
		var e error
		taskUUID, e = p.client.CloneVM(ctx, body)
		return e
	})
	if err != nil {
		return nil, provider.Wrap("nutanix", "CreateVM", provider.CodeInternal,
			"failed to create VM", err)
	}

	// Wait for the create task and get the new VM UUID.
	vmUUID, err := waitForTaskWithResult(ctx, p.client, taskUUID, taskPollConfig{
		interval: 3 * time.Second,
		timeout:  10 * time.Minute,
	})
	if err != nil {
		return nil, provider.Wrap("nutanix", "CreateVM", provider.CodeInternal,
			"VM creation task failed", err)
	}

	p.log.Info("nutanix: VM created", logger.String("vm_id", vmUUID))
	return p.GetVM(ctx, vmUUID)
}

// DeleteVM deletes a VM and waits for the task to complete.
func (p *Provider) DeleteVM(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	// Power off first if running.
	if err := p.powerOffIfRunning(ctx, providerVMID); err != nil {
		p.log.Warn("nutanix: could not power off VM before delete (proceeding anyway)",
			logger.String("vm_id", providerVMID), logger.Error(err))
	}

	taskUUID, err := p.client.DeleteVM(ctx, providerVMID)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return provider.New("nutanix", "DeleteVM", provider.CodeNotFound,
				fmt.Sprintf("VM %q not found", providerVMID))
		}
		return provider.Wrap("nutanix", "DeleteVM", provider.CodeInternal,
			"failed to delete VM", err)
	}

	if err := waitForTask(ctx, p.client, taskUUID, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("nutanix", "DeleteVM", provider.CodeInternal,
			"VM deletion task failed", err)
	}

	p.log.Info("nutanix: VM deleted", logger.String("vm_id", providerVMID))
	return nil
}

// CloneVM creates a full clone of an existing VM.
func (p *Provider) CloneVM(ctx context.Context, providerVMID string, spec port.VMCloneSpec) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	sourceVM, err := p.client.GetVM(ctx, providerVMID)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return nil, provider.New("nutanix", "CloneVM", provider.CodeNotFound,
				fmt.Sprintf("source VM %q not found", providerVMID))
		}
		return nil, provider.Wrap("nutanix", "CloneVM", provider.CodeInternal,
			"failed to get source VM", err)
	}

	body := buildVMCloneSpec(sourceVM, spec)

	var taskUUID string
	err = withOperationRetry(ctx, func(_ int) error {
		var e error
		taskUUID, e = p.client.CloneVM(ctx, body)
		return e
	})
	if err != nil {
		return nil, provider.Wrap("nutanix", "CloneVM", provider.CodeInternal,
			"failed to clone VM", err)
	}

	pollCfg := taskPollConfig{interval: 3 * time.Second, timeout: 15 * time.Minute}
	newVMUUID, err := waitForTaskWithResult(ctx, p.client, taskUUID, pollCfg)
	if err != nil {
		return nil, provider.Wrap("nutanix", "CloneVM", provider.CodeInternal,
			"VM clone task failed", err)
	}

	p.log.Info("nutanix: VM cloned",
		logger.String("source_vm_id", providerVMID),
		logger.String("new_vm_id", newVMUUID),
	)
	return p.GetVM(ctx, newVMUUID)
}

// ── Power Operations ─────────────────────────────────────────────────────────

// PowerOn starts a stopped VM.
func (p *Provider) PowerOn(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "ON", "PowerOn")
}

// PowerOff hard-stops a running VM.
func (p *Provider) PowerOff(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "OFF", "PowerOff")
}

// Reboot issues a graceful ACPI reboot to the guest OS.
func (p *Provider) Reboot(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "ACPI_REBOOT", "Reboot")
}

// Suspend pauses the VM.
func (p *Provider) Suspend(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "PAUSE", "Suspend")
}

// Reset performs a hard power cycle.
func (p *Provider) Reset(ctx context.Context, providerVMID string) error {
	return p.powerAction(ctx, providerVMID, "POWERCYCLE", "Reset")
}

// powerAction is the shared implementation for all power state transitions.
func (p *Provider) powerAction(ctx context.Context, providerVMID, transition, opName string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	var taskUUID string
	err := withOperationRetry(ctx, func(_ int) error {
		var e error
		taskUUID, e = p.client.VMPowerAction(ctx, providerVMID, transition)
		return e
	})
	if err != nil {
		if ae, ok := err.(*apiError); ok {
			if ae.isNotFound() {
				return provider.New("nutanix", opName, provider.CodeNotFound,
					fmt.Sprintf("VM %q not found", providerVMID))
			}
		}
		return provider.Wrap("nutanix", opName, provider.CodeInternal,
			fmt.Sprintf("power action %q failed", transition), err)
	}

	if err := waitForTask(ctx, p.client, taskUUID, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("nutanix", opName, provider.CodeInternal,
			fmt.Sprintf("power action %q task failed", transition), err)
	}

	p.log.Info("nutanix: power action completed",
		logger.String("vm_id", providerVMID),
		logger.String("transition", transition),
	)
	return nil
}

// ── Metrics ──────────────────────────────────────────────────────────────────

// GetVMMetrics returns empty metrics — Nutanix detailed metrics require Prism Pro.
// The capability flag GuestMetrics=false signals callers not to rely on this.
func (p *Provider) GetVMMetrics(_ context.Context, _ string) (*port.VMMetrics, error) {
	return &port.VMMetrics{}, nil
}

// ── Snapshots ────────────────────────────────────────────────────────────────

// ListSnapshots returns all recovery points for a VM.
func (p *Provider) ListSnapshots(ctx context.Context, providerVMID string) ([]port.SnapshotInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	snaps, err := p.client.ListVMSnapshots(ctx, providerVMID)
	if err != nil {
		return nil, provider.Wrap("nutanix", "ListSnapshots", provider.CodeInternal,
			"failed to list snapshots", err)
	}

	infos := make([]port.SnapshotInfo, 0, len(snaps))
	for i := range snaps {
		infos = append(infos, mapSnapshot(&snaps[i]))
	}
	return infos, nil
}

// CreateSnapshot creates a recovery point for a VM.
func (p *Provider) CreateSnapshot(ctx context.Context, providerVMID string, spec port.SnapshotSpec) (*port.SnapshotInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	var taskUUID string
	err := withOperationRetry(ctx, func(_ int) error {
		var e error
		taskUUID, e = p.client.CreateVMSnapshot(ctx, providerVMID, spec.Name)
		return e
	})
	if err != nil {
		return nil, provider.Wrap("nutanix", "CreateSnapshot", provider.CodeInternal,
			"failed to create snapshot", err)
	}

	snapshotUUID, err := waitForTaskWithResult(ctx, p.client, taskUUID, defaultTaskPollConfig())
	if err != nil {
		return nil, provider.Wrap("nutanix", "CreateSnapshot", provider.CodeInternal,
			"snapshot task failed", err)
	}

	info := &port.SnapshotInfo{
		ProviderID:  snapshotUUID,
		Name:        spec.Name,
		Description: spec.Description,
		IsCurrent:   false,
		CreatedAt:   timeNow(),
	}
	p.log.Info("nutanix: snapshot created",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot", spec.Name),
	)
	return info, nil
}

// DeleteSnapshot deletes a recovery point by UUID.
func (p *Provider) DeleteSnapshot(ctx context.Context, _ string, snapshotID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	taskUUID, err := p.client.DeleteVMSnapshot(ctx, snapshotID)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return provider.New("nutanix", "DeleteSnapshot", provider.CodeNotFound,
				fmt.Sprintf("snapshot %q not found", snapshotID))
		}
		return provider.Wrap("nutanix", "DeleteSnapshot", provider.CodeInternal,
			"failed to delete snapshot", err)
	}

	if err := waitForTask(ctx, p.client, taskUUID, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("nutanix", "DeleteSnapshot", provider.CodeInternal,
			"snapshot deletion task failed", err)
	}

	p.log.Info("nutanix: snapshot deleted", logger.String("snapshot_id", snapshotID))
	return nil
}

// RevertSnapshot restores a VM from a recovery point.
func (p *Provider) RevertSnapshot(ctx context.Context, providerVMID, snapshotID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	taskUUID, err := p.client.RestoreVMSnapshot(ctx, providerVMID, snapshotID)
	if err != nil {
		if ae, ok := err.(*apiError); ok && ae.isNotFound() {
			return provider.New("nutanix", "RevertSnapshot", provider.CodeNotFound,
				fmt.Sprintf("snapshot %q not found", snapshotID))
		}
		return provider.Wrap("nutanix", "RevertSnapshot", provider.CodeInternal,
			"failed to revert snapshot", err)
	}

	if err := waitForTask(ctx, p.client, taskUUID, defaultTaskPollConfig()); err != nil {
		return provider.Wrap("nutanix", "RevertSnapshot", provider.CodeInternal,
			"snapshot revert task failed", err)
	}

	p.log.Info("nutanix: snapshot reverted",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot_id", snapshotID),
	)
	return nil
}

// ── Storage ──────────────────────────────────────────────────────────────────

// ListDataStores returns all Nutanix storage containers.
// Note: Nutanix storage containers are accessed via the v2 API path on Prism Element.
// For Prism Central, this falls back to an empty list with a warning.
func (p *Provider) ListDataStores(ctx context.Context) ([]port.DataStoreInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	// Nutanix storage containers are available via a different API path.
	// We use a best-effort approach — if the endpoint is unavailable (Prism Central),
	// we return an empty list rather than failing the entire sync.
	var containers []nutanixStorageContainer
	err := p.client.get(ctx, "/storage_containers", &struct {
		Entities *[]nutanixStorageContainer `json:"entities"`
	}{Entities: &containers})
	if err != nil {
		// Non-fatal: Prism Central may not expose this endpoint.
		p.log.Warn("nutanix: failed to list storage containers (may be Prism Central)",
			logger.Error(err))
		return []port.DataStoreInfo{}, nil
	}

	infos := make([]port.DataStoreInfo, 0, len(containers))
	for i := range containers {
		infos = append(infos, mapStorageContainer(&containers[i]))
	}
	return infos, nil
}

// GetDataStore retrieves a single storage container by its UUID or name.
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
	return nil, provider.New("nutanix", "GetDataStore", provider.CodeNotFound,
		fmt.Sprintf("storage container %q not found", id))
}

// ── Networks ─────────────────────────────────────────────────────────────────

// ListNetworks returns all Nutanix subnets.
func (p *Provider) ListNetworks(ctx context.Context) ([]port.NetworkInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	subnets, err := p.client.ListSubnets(ctx)
	if err != nil {
		return nil, provider.Wrap("nutanix", "ListNetworks", provider.CodeInternal,
			"failed to list subnets", err)
	}

	infos := make([]port.NetworkInfo, 0, len(subnets))
	for i := range subnets {
		infos = append(infos, mapSubnet(&subnets[i]))
	}
	return infos, nil
}

// GetNetwork retrieves a single subnet by its UUID or name.
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
	return nil, provider.New("nutanix", "GetNetwork", provider.CodeNotFound,
		fmt.Sprintf("subnet %q not found", id))
}

// ── Inventory Sync ───────────────────────────────────────────────────────────

// SyncInventory fetches the full live inventory from Nutanix using concurrent
// goroutines for VMs, storage, and networks, then returns a normalised snapshot.
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
		return nil, fmt.Errorf("nutanix SyncInventory ListVMs: %w", vmRes.err)
	}
	// DataStore and Network errors are non-fatal — log and continue.
	if dsRes.err != nil {
		p.log.Warn("nutanix: SyncInventory ListDataStores failed (non-fatal)",
			logger.Error(dsRes.err))
		dsRes.data = []port.DataStoreInfo{}
	}
	if netRes.err != nil {
		p.log.Warn("nutanix: SyncInventory ListNetworks failed (non-fatal)",
			logger.Error(netRes.err))
		netRes.data = []port.NetworkInfo{}
	}

	snap := &port.InventorySnapshot{
		VMs:        vmRes.data,
		DataStores: dsRes.data,
		Networks:   netRes.data,
		SyncedAt:   timeNow(),
	}
	p.log.Info("nutanix: inventory sync complete",
		logger.Int("vms", len(snap.VMs)),
		logger.Int("datastores", len(snap.DataStores)),
		logger.Int("networks", len(snap.Networks)),
	)
	return snap, nil
}

// ── Templates ────────────────────────────────────────────────────────────────

// ListTemplates returns all Nutanix disk images that can be used as VM templates.
func (p *Provider) ListTemplates(ctx context.Context) ([]port.TemplateInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	images, err := p.client.ListImages(ctx)
	if err != nil {
		return nil, provider.Wrap("nutanix", "ListTemplates", provider.CodeInternal,
			"failed to list images", err)
	}

	var infos []port.TemplateInfo
	for i := range images {
		img := &images[i]
		// Only include DISK_IMAGE type — ISO images are not VM templates.
		if img.Status.Resources.ImageType != "DISK_IMAGE" {
			continue
		}
		infos = append(infos, mapImageToTemplate(img))
	}

	p.log.Info("nutanix: listed templates", logger.Int("count", len(infos)))
	return infos, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// buildTopologyMaps fetches clusters and hosts and returns UUID→name maps.
// Failures are non-fatal — returns empty maps on error.
func (p *Provider) buildTopologyMaps(ctx context.Context) (clusterMap, hostMap map[string]string) {
	clusterMap = make(map[string]string)
	hostMap = make(map[string]string)

	clusters, err := p.client.ListClusters(ctx)
	if err != nil {
		p.log.Warn("nutanix: failed to fetch clusters for topology enrichment", logger.Error(err))
	} else {
		for _, c := range clusters {
			clusterMap[c.Metadata.UUID] = c.Status.Name
		}
	}

	hosts, err := p.client.ListHosts(ctx)
	if err != nil {
		p.log.Warn("nutanix: failed to fetch hosts for topology enrichment", logger.Error(err))
	} else {
		for _, h := range hosts {
			hostMap[h.Metadata.UUID] = h.Status.Name
		}
	}

	return clusterMap, hostMap
}

// pickCluster returns the UUID of the first available cluster.
func (p *Provider) pickCluster(ctx context.Context) (string, error) {
	clusters, err := p.client.ListClusters(ctx)
	if err != nil {
		return "", provider.Wrap("nutanix", "pickCluster", provider.CodeInternal,
			"failed to list clusters", err)
	}
	for _, c := range clusters {
		if c.Status.State == "COMPLETE" || c.Status.State == "ACTIVE" || c.Status.State == "" {
			return c.Metadata.UUID, nil
		}
	}
	if len(clusters) > 0 {
		return clusters[0].Metadata.UUID, nil
	}
	return "", provider.New("nutanix", "pickCluster", provider.CodeInternal,
		"no clusters available")
}

// powerOffIfRunning powers off a VM if it is currently running.
func (p *Provider) powerOffIfRunning(ctx context.Context, providerVMID string) error {
	vm, err := p.client.GetVM(ctx, providerVMID)
	if err != nil {
		return err
	}
	if vm.Status.Resources.PowerState != "ON" {
		return nil
	}
	taskUUID, err := p.client.VMPowerAction(ctx, providerVMID, "OFF")
	if err != nil {
		return err
	}
	return waitForTask(ctx, p.client, taskUUID, defaultTaskPollConfig())
}
