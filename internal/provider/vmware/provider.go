// Package vmware implements the port.Provider interface for VMware vSphere / vCenter
// using the govmomi SDK. All vCenter API calls are made through a managed
// *govmomi.Client that is created in Connect and torn down in Disconnect.
//
// Architecture:
//
//	provider.go   – port.Provider + port.ConsoleProvider implementation
//	client.go     – connection manager (govmomi.Client lifecycle, session keep-alive)
//	mapper.go     – govmomi ManagedObject → port.* DTO conversions
//	retry.go      – exponential-backoff retry helper
package vmware

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
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
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

// Provider implements port.Provider and port.ConsoleProvider for VMware vSphere.
type Provider struct {
	provider.BaseProvider
	cfg    config.VMwareConfig
	log    logger.Logger
	conn   *connectionManager // nil until Connect is called
}

// NewProvider creates a new VMware provider instance.
// The provider is not connected until Connect is called.
func NewProvider(cfg config.VMwareConfig, log logger.Logger) *Provider {
	return &Provider{cfg: cfg, log: log}
}

// Client returns the underlying govmomi vim25.Client for use by sub-providers
// (e.g. the ESXi provider) that need direct SDK access.
// Returns nil if not connected.
func (p *Provider) Client() *govmomi.Client {
	if p.conn == nil {
		return nil
	}
	return p.conn.client
}

// ── Meta ─────────────────────────────────────────────────────────────────────

func (p *Provider) Type() model.ProviderType { return model.ProviderVMware }
func (p *Provider) Name() string             { return "VMware vSphere" }

// Capabilities declares the full vSphere feature set.
func (p *Provider) Capabilities() port.ProviderCapabilities {
	return port.ProviderCapabilities{
		Console:          true, // WebMKS via AcquireTicket
		LinkedClones:     true,
		MemorySnapshots:  true,
		QuiesceSnapshots: true,
		LiveMigration:    true, // vMotion
		GuestMetrics:     true, // Performance Manager
		TemplateClone:    true,
	}
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

// Connect authenticates to vCenter and stores the live govmomi client.
// It retries the initial connection up to maxConnectAttempts times with
// exponential backoff to handle transient network issues at startup.
func (p *Provider) Connect(ctx context.Context, creds port.Credentials) error {
	p.log.Info("vmware: connecting",
		logger.String("host", creds.Host),
		logger.Bool("tls_verify", creds.TLSVerify),
	)

	timeout := p.cfg.DefaultTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var conn *connectionManager
	err := withRetry(ctx, retryConfig{
		maxAttempts: 3,
		baseDelay:   2 * time.Second,
		maxDelay:    10 * time.Second,
		shouldRetry: isRetryableConnectError,
	}, func(attempt int) error {
		if attempt > 1 {
			p.log.Warn("vmware: retrying connection",
				logger.String("host", creds.Host),
				logger.Int("attempt", attempt),
			)
		}
		var err error
		conn, err = newConnectionManager(ctx, creds, timeout)
		return err
	})
	if err != nil {
		return provider.Wrap("vmware", "Connect", provider.CodeAuthFailed,
			fmt.Sprintf("failed to connect to vCenter at %s", creds.Host), err)
	}

	p.conn = conn
	p.StoreCredentials(creds)
	p.SetConnected(true)
	p.log.Info("vmware: connected",
		logger.String("host", creds.Host),
		logger.String("version", conn.serviceVersion()),
	)
	return nil
}

// Disconnect logs out of vCenter and releases the govmomi client.
func (p *Provider) Disconnect(ctx context.Context) error {
	if p.conn != nil {
		if err := p.conn.logout(ctx); err != nil {
			p.log.Warn("vmware: logout error (ignored)", logger.Error(err))
		}
		p.conn = nil
	}
	p.SetConnected(false)
	p.log.Info("vmware: disconnected")
	return nil
}

// Ping verifies the vCenter session is still active.
func (p *Provider) Ping(ctx context.Context) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	if err := p.conn.ping(ctx); err != nil {
		p.SetConnected(false)
		return provider.Wrap("vmware", "Ping", provider.CodeNotConnected, "session check failed", err)
	}
	return nil
}

// ── VM Operations ────────────────────────────────────────────────────────────

// ListVMs returns all VMs visible to the authenticated user via a ContainerView.
// Using a ContainerView is significantly more efficient than recursive folder
// traversal because it issues a single RetrievePropertiesEx RPC.
func (p *Provider) ListVMs(ctx context.Context) ([]port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	m := view.NewManager(p.conn.client.Client)
	cv, err := m.CreateContainerView(ctx, p.conn.client.ServiceContent.RootFolder,
		[]string{"VirtualMachine"}, true)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListVMs", provider.CodeInternal,
			"failed to create container view", err)
	}
	defer cv.Destroy(ctx) //nolint:errcheck

	var moVMs []mo.VirtualMachine
	err = cv.Retrieve(ctx, []string{"VirtualMachine"}, VMProperties, &moVMs)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListVMs", provider.CodeInternal,
			"failed to retrieve VM properties", err)
	}

	infos := make([]port.VMInfo, 0, len(moVMs))
	for i := range moVMs {
		infos = append(infos, mapVMInfo(&moVMs[i]))
	}
	p.log.Info("vmware: listed VMs", logger.Int("count", len(infos)))
	return infos, nil
}

// GetVM retrieves a single VM by its ManagedObject reference string (e.g. "vm-42").
func (p *Provider) GetVM(ctx context.Context, providerVMID string) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return nil, err
	}

	var moVM mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(), VMProperties, &moVM); err != nil {
		return nil, provider.Wrap("vmware", "GetVM", provider.CodeInternal,
			"failed to retrieve VM properties", err)
	}

	info := mapVMInfo(&moVM)
	return &info, nil
}

// CreateVM deploys a new VM from a template or from scratch.
// When spec.TemplateID is set the VM is cloned from that template;
// otherwise a minimal blank VM is created (requires further customisation).
func (p *Provider) CreateVM(ctx context.Context, spec port.VMCreateSpec) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	finder := find.NewFinder(p.conn.client.Client, true)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"no default datacenter", err)
	}
	finder.SetDatacenter(dc)

	if spec.TemplateID != "" {
		return p.cloneFromTemplate(ctx, finder, spec)
	}
	return p.createBlankVM(ctx, finder, spec)
}

// DeleteVM destroys a VM and removes it from disk.
func (p *Provider) DeleteVM(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}

	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}

	// Power off first if running.
	var moVM mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"runtime.powerState"}, &moVM); err == nil {
		if moVM.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
			task, err := vm.PowerOff(ctx)
			if err == nil {
				_ = task.Wait(ctx)
			}
		}
	}

	task, err := vm.Destroy(ctx)
	if err != nil {
		return provider.Wrap("vmware", "DeleteVM", provider.CodeInternal,
			"failed to initiate destroy", err)
	}
	if err := task.Wait(ctx); err != nil {
		return provider.Wrap("vmware", "DeleteVM", provider.CodeInternal,
			"destroy task failed", err)
	}
	p.log.Info("vmware: VM deleted", logger.String("vm_id", providerVMID))
	return nil
}

// CloneVM creates a full or linked clone of an existing VM or template.
//
// vSphere requirements:
//   - A ResourcePool is mandatory for cloning both regular VMs and templates.
//     Without it vCenter returns "The operation is not supported on the object."
//   - Full clone: DiskMoveType must be moveAllDiskBackingsAndConsolidate.
//   - Linked clone: requires an existing snapshot; DiskMoveType must be
//     createNewChildDiskBacking.
//   - When the source is a template (config.template=true), its ResourcePool
//     field is nil. We must resolve the pool from the host the template lives on.
func (p *Provider) CloneVM(ctx context.Context, providerVMID string, spec port.VMCloneSpec) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	finder := find.NewFinder(p.conn.client.Client, true)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CloneVM", provider.CodeInternal,
			"no default datacenter", err)
	}
	finder.SetDatacenter(dc)

	// Fetch the source VM/template properties we need in one call.
	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: providerVMID}
	var moSrc mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, ref, []string{
		"config.name", "config.template", "resourcePool", "runtime.host", "parent",
	}, &moSrc); err != nil {
		return nil, provider.Wrap("vmware", "CloneVM", provider.CodeNotFound,
			fmt.Sprintf("source VM/template %q not found", providerVMID), err)
	}

	srcVM := object.NewVirtualMachine(p.conn.client.Client, ref)
	isTemplate := moSrc.Config != nil && moSrc.Config.Template

	// ── Resolve destination folder ────────────────────────────────────────────
	// Use the source object's parent folder; fall back to the default folder.
	var folder *object.Folder
	if moSrc.Parent != nil && moSrc.Parent.Type == "Folder" {
		folder = object.NewFolder(p.conn.client.Client, *moSrc.Parent)
	} else {
		folder, err = finder.DefaultFolder(ctx)
		if err != nil {
			return nil, provider.Wrap("vmware", "CloneVM", provider.CodeInternal,
				"could not resolve destination folder", err)
		}
	}

	// ── Resolve resource pool ─────────────────────────────────────────────────
	// Regular VMs have ResourcePool set directly.
	// Templates have ResourcePool=nil — resolve via the host they live on.
	var poolRef types.ManagedObjectReference
	switch {
	case !isTemplate && moSrc.ResourcePool != nil:
		// Regular VM: use its own resource pool.
		poolRef = *moSrc.ResourcePool

	case moSrc.Runtime.Host != nil:
		// Template (or VM with no pool): find the resource pool of the host.
		hostPool, poolErr := p.hostResourcePool(ctx, *moSrc.Runtime.Host)
		if poolErr != nil {
			return nil, provider.Wrap("vmware", "CloneVM", provider.CodeInternal,
				"could not resolve resource pool from host", poolErr)
		}
		poolRef = hostPool

	default:
		// Last resort: use the default resource pool from the finder.
		defaultPool, poolErr := finder.DefaultResourcePool(ctx)
		if poolErr != nil {
			return nil, provider.Wrap("vmware", "CloneVM", provider.CodeInternal,
				"could not find any resource pool", poolErr)
		}
		poolRef = defaultPool.Reference()
	}

	relocSpec := types.VirtualMachineRelocateSpec{
		Pool: &poolRef,
		// Full clone: consolidate all disk backings into a single independent disk.
		DiskMoveType: string(types.VirtualMachineRelocateDiskMoveOptionsMoveAllDiskBackingsAndConsolidate),
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location: relocSpec,
		PowerOn:  false,
		Template: false,
	}

	if spec.Linked {
		// Linked clone: must have a snapshot to use as the base delta.
		snapRef := p.currentSnapshotRef(ctx, srcVM)
		if snapRef == nil {
			return nil, provider.New("vmware", "CloneVM", provider.CodeInvalidState,
				"linked clone requires at least one snapshot on the source VM")
		}
		cloneSpec.Snapshot = snapRef
		cloneSpec.Location.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsCreateNewChildDiskBacking)
	}

	if spec.DataStore != "" {
		ds, err := finder.Datastore(ctx, spec.DataStore)
		if err == nil {
			ref := ds.Reference()
			cloneSpec.Location.Datastore = &ref
		} else {
			p.log.Warn("vmware: CloneVM datastore not found, using source datastore",
				logger.String("datastore", spec.DataStore), logger.Error(err))
		}
	}

	p.log.Info("vmware: starting clone",
		logger.String("src_vm_id", providerVMID),
		logger.String("name", spec.Name),
		logger.Bool("is_template", isTemplate),
		logger.String("pool_ref", poolRef.Value),
	)

	task, err := srcVM.Clone(ctx, folder, spec.Name, cloneSpec)
	if err != nil {
		return nil, provider.Wrap("vmware", "CloneVM", provider.CodeInternal,
			"clone initiation failed", err)
	}

	info, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CloneVM", provider.CodeInternal,
			"clone task failed", err)
	}

	newVM := object.NewVirtualMachine(p.conn.client.Client, info.Result.(types.ManagedObjectReference))
	return p.GetVM(ctx, newVM.Reference().Value)
}

// ── Power Operations ─────────────────────────────────────────────────────────

// PowerOn starts a VM. Returns ErrInvalidState if already running.
func (p *Provider) PowerOn(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}
	if err := p.assertPowerState(ctx, vm, types.VirtualMachinePowerStatePoweredOff,
		types.VirtualMachinePowerStateSuspended); err != nil {
		return err
	}
	task, err := vm.PowerOn(ctx)
	if err != nil {
		return provider.Wrap("vmware", "PowerOn", provider.CodeInternal, "power-on failed", err)
	}
	return p.waitTask(ctx, "PowerOn", providerVMID, task)
}

// PowerOff hard-stops a VM (equivalent to pulling the power cord).
// Use Reboot for a graceful guest shutdown.
func (p *Provider) PowerOff(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}
	if err := p.assertPowerState(ctx, vm, types.VirtualMachinePowerStatePoweredOn); err != nil {
		return err
	}
	task, err := vm.PowerOff(ctx)
	if err != nil {
		return provider.Wrap("vmware", "PowerOff", provider.CodeInternal, "power-off failed", err)
	}
	return p.waitTask(ctx, "PowerOff", providerVMID, task)
}

// Reboot issues a guest OS reboot via VMware Tools (graceful).
// Falls back to a hard reset if Tools are not running.
func (p *Provider) Reboot(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}
	if err := p.assertPowerState(ctx, vm, types.VirtualMachinePowerStatePoweredOn); err != nil {
		return err
	}
	// Try graceful guest reboot first.
	if err := vm.RebootGuest(ctx); err != nil {
		p.log.Warn("vmware: guest reboot failed, falling back to hard reset",
			logger.String("vm_id", providerVMID), logger.Error(err))
		task, err := vm.Reset(ctx)
		if err != nil {
			return provider.Wrap("vmware", "Reboot", provider.CodeInternal, "reset failed", err)
		}
		return p.waitTask(ctx, "Reboot/Reset", providerVMID, task)
	}
	p.log.Info("vmware: guest reboot issued", logger.String("vm_id", providerVMID))
	return nil
}

// Suspend saves the VM memory state to disk (suspend-to-disk).
func (p *Provider) Suspend(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}
	if err := p.assertPowerState(ctx, vm, types.VirtualMachinePowerStatePoweredOn); err != nil {
		return err
	}
	task, err := vm.Suspend(ctx)
	if err != nil {
		return provider.Wrap("vmware", "Suspend", provider.CodeInternal, "suspend failed", err)
	}
	return p.waitTask(ctx, "Suspend", providerVMID, task)
}

// Reset performs a hard reset (power cycle) without saving state.
func (p *Provider) Reset(ctx context.Context, providerVMID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}
	if err := p.assertPowerState(ctx, vm, types.VirtualMachinePowerStatePoweredOn); err != nil {
		return err
	}
	task, err := vm.Reset(ctx)
	if err != nil {
		return provider.Wrap("vmware", "Reset", provider.CodeInternal, "reset failed", err)
	}
	return p.waitTask(ctx, "Reset", providerVMID, task)
}

// ── Metrics ──────────────────────────────────────────────────────────────────

// GetVMMetrics queries the vSphere Performance Manager for real-time counters.
// It uses a 20-second sample interval (the finest available in real-time).
func (p *Provider) GetVMMetrics(ctx context.Context, providerVMID string) (*port.VMMetrics, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return nil, err
	}
	return collectVMMetrics(ctx, p.conn.client.Client, vm.Reference())
}

// ── Snapshots ────────────────────────────────────────────────────────────────

// ListSnapshots returns the snapshot tree for a VM flattened into a slice.
func (p *Provider) ListSnapshots(ctx context.Context, providerVMID string) ([]port.SnapshotInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	var moVM mo.VirtualMachine
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return nil, err
	}
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(),
		[]string{"snapshot", "config.name"}, &moVM); err != nil {
		return nil, provider.Wrap("vmware", "ListSnapshots", provider.CodeInternal,
			"failed to retrieve snapshot tree", err)
	}

	if moVM.Snapshot == nil {
		return []port.SnapshotInfo{}, nil
	}

	var currentRef *types.ManagedObjectReference
	if moVM.Snapshot.CurrentSnapshot != nil {
		currentRef = moVM.Snapshot.CurrentSnapshot
	}

	return flattenSnapshotTree(moVM.Snapshot.RootSnapshotList, currentRef, ""), nil
}

// CreateSnapshot takes a snapshot of the VM with optional memory and quiesce.
func (p *Provider) CreateSnapshot(ctx context.Context, providerVMID string, spec port.SnapshotSpec) (*port.SnapshotInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return nil, err
	}

	task, err := vm.CreateSnapshot(ctx, spec.Name, spec.Description, spec.Memory, spec.Quiesce)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateSnapshot", provider.CodeInternal,
			"snapshot creation failed", err)
	}
	result, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateSnapshot", provider.CodeInternal,
			"snapshot task failed", err)
	}

	snapRef := result.Result.(types.ManagedObjectReference)
	info := &port.SnapshotInfo{
		ProviderID:  snapRef.Value,
		Name:        spec.Name,
		Description: spec.Description,
		IsCurrent:   true,
		CreatedAt:   timeNow(),
	}
	p.log.Info("vmware: snapshot created",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot_id", snapRef.Value),
	)
	return info, nil
}

// DeleteSnapshot removes a snapshot by its ManagedObject reference value.
// Child snapshots are consolidated (removeChildren=false).
func (p *Provider) DeleteSnapshot(ctx context.Context, providerVMID, snapshotID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}

	// govmomi RemoveSnapshot operates by snapshot name, not MOR value.
	// consolidate=true merges the delta disks after removal.
	consolidate := types.NewBool(true)
	task, err := vm.RemoveSnapshot(ctx, snapshotID, false, consolidate)
	if err != nil {
		return provider.Wrap("vmware", "DeleteSnapshot", provider.CodeInternal,
			"snapshot removal failed", err)
	}
	if err := p.waitTask(ctx, "DeleteSnapshot", providerVMID, task); err != nil {
		return err
	}
	p.log.Info("vmware: snapshot deleted",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot_id", snapshotID),
	)
	return nil
}

// RevertSnapshot reverts the VM to the named snapshot.
func (p *Provider) RevertSnapshot(ctx context.Context, providerVMID, snapshotID string) error {
	if err := p.EnsureConnected(ctx); err != nil {
		return err
	}
	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return err
	}

	// suppressPowerOn=false: restore the power state the VM had when snapshotted.
	task, err := vm.RevertToSnapshot(ctx, snapshotID, false)
	if err != nil {
		return provider.Wrap("vmware", "RevertSnapshot", provider.CodeInternal,
			"snapshot revert failed", err)
	}
	if err := p.waitTask(ctx, "RevertSnapshot", providerVMID, task); err != nil {
		return err
	}
	p.log.Info("vmware: snapshot reverted",
		logger.String("vm_id", providerVMID),
		logger.String("snapshot_id", snapshotID),
	)
	return nil
}

// ── Storage ──────────────────────────────────────────────────────────────────

// ListDataStores returns all datastores visible in the default datacenter.
func (p *Provider) ListDataStores(ctx context.Context) ([]port.DataStoreInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	m := view.NewManager(p.conn.client.Client)
	cv, err := m.CreateContainerView(ctx, p.conn.client.ServiceContent.RootFolder,
		[]string{"Datastore"}, true)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListDataStores", provider.CodeInternal,
			"failed to create container view", err)
	}
	defer cv.Destroy(ctx) //nolint:errcheck

	var moDSs []mo.Datastore
	err = cv.Retrieve(ctx, []string{"Datastore"},
		[]string{"summary", "info"}, &moDSs)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListDataStores", provider.CodeInternal,
			"failed to retrieve datastore properties", err)
	}

	infos := make([]port.DataStoreInfo, 0, len(moDSs))
	for i := range moDSs {
		infos = append(infos, mapDataStoreInfo(&moDSs[i]))
	}
	return infos, nil
}

// GetDataStore retrieves a single datastore by its ManagedObject reference value.
func (p *Provider) GetDataStore(ctx context.Context, id string) (*port.DataStoreInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	ref := types.ManagedObjectReference{Type: "Datastore", Value: id}
	var moDS mo.Datastore
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, ref, []string{"summary", "info"}, &moDS); err != nil {
		return nil, provider.Wrap("vmware", "GetDataStore", provider.CodeNotFound,
			fmt.Sprintf("datastore %q not found", id), err)
	}
	info := mapDataStoreInfo(&moDS)
	return &info, nil
}

// ── Networks ─────────────────────────────────────────────────────────────────

// ListNetworks returns all standard and distributed port groups.
func (p *Provider) ListNetworks(ctx context.Context) ([]port.NetworkInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	m := view.NewManager(p.conn.client.Client)
	cv, err := m.CreateContainerView(ctx, p.conn.client.ServiceContent.RootFolder,
		[]string{"Network", "DistributedVirtualPortgroup"}, true)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListNetworks", provider.CodeInternal,
			"failed to create container view", err)
	}
	defer cv.Destroy(ctx) //nolint:errcheck

	var moNets []mo.Network
	err = cv.Retrieve(ctx, []string{"Network"}, []string{"summary", "name"}, &moNets)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListNetworks", provider.CodeInternal,
			"failed to retrieve network properties", err)
	}

	infos := make([]port.NetworkInfo, 0, len(moNets))
	for i := range moNets {
		infos = append(infos, mapNetworkInfo(&moNets[i]))
	}
	return infos, nil
}

// GetNetwork retrieves a single network by its ManagedObject reference value.
func (p *Provider) GetNetwork(ctx context.Context, id string) (*port.NetworkInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	ref := types.ManagedObjectReference{Type: "Network", Value: id}
	var moNet mo.Network
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, ref, []string{"summary", "name"}, &moNet); err != nil {
		return nil, provider.Wrap("vmware", "GetNetwork", provider.CodeNotFound,
			fmt.Sprintf("network %q not found", id), err)
	}
	info := mapNetworkInfo(&moNet)
	return &info, nil
}

// ── Inventory Sync ───────────────────────────────────────────────────────────

// SyncInventory fetches the full live inventory from vCenter in a single pass.
// It runs VM, datastore, network, host, and cluster retrievals concurrently,
// then enriches each VM with its ESXi host name, cluster name, and datastore
// name before returning the normalised snapshot.
func (p *Provider) SyncInventory(ctx context.Context) (*port.InventorySnapshot, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	type vmResult struct {
		vms  []mo.VirtualMachine
		err  error
	}
	type dsResult struct {
		data []port.DataStoreInfo
		err  error
	}
	type netResult struct {
		data []port.NetworkInfo
		err  error
	}
	type hostResult struct {
		hosts    map[string]string // MOR → name (kept for topology lookups)
		moHosts  []mo.HostSystem   // full objects for hardware mapping
		err      error
	}
	type clusterResult struct {
		clusters map[string]string // MOR → name
		err      error
	}

	vmCh      := make(chan vmResult, 1)
	dsCh      := make(chan dsResult, 1)
	netCh     := make(chan netResult, 1)
	hostCh    := make(chan hostResult, 1)
	clusterCh := make(chan clusterResult, 1)

	m := view.NewManager(p.conn.client.Client)

	// VMs — raw mo objects so we can enrich with topology.
	go func() {
		cv, err := m.CreateContainerView(ctx, p.conn.client.ServiceContent.RootFolder,
			[]string{"VirtualMachine"}, true)
		if err != nil {
			vmCh <- vmResult{err: err}
			return
		}
		defer cv.Destroy(ctx) //nolint:errcheck
		var moVMs []mo.VirtualMachine
		if err := cv.Retrieve(ctx, []string{"VirtualMachine"}, VMProperties, &moVMs); err != nil {
			vmCh <- vmResult{err: err}
			return
		}
		vmCh <- vmResult{vms: moVMs}
	}()

	// Datastores.
	go func() {
		ds, err := p.ListDataStores(ctx)
		dsCh <- dsResult{data: ds, err: err}
	}()

	// Networks.
	go func() {
		nets, err := p.ListNetworks(ctx)
		netCh <- netResult{data: nets, err: err}
	}()

	// ESXi hosts — build MOR→name map.
	go func() {
		cv, err := m.CreateContainerView(ctx, p.conn.client.ServiceContent.RootFolder,
			[]string{"HostSystem"}, true)
		if err != nil {
			hostCh <- hostResult{err: err}
			return
		}
		defer cv.Destroy(ctx) //nolint:errcheck
		var moHosts []mo.HostSystem
		if err := cv.Retrieve(ctx, []string{"HostSystem"}, hostProperties, &moHosts); err != nil {
			hostCh <- hostResult{err: err}
			return
		}
		hosts := make(map[string]string, len(moHosts))
		for _, h := range moHosts {
			hosts[h.Self.Value] = h.Name
		}
		hostCh <- hostResult{hosts: hosts, moHosts: moHosts}
	}()

	// Clusters — build MOR→name map.
	go func() {
		cv, err := m.CreateContainerView(ctx, p.conn.client.ServiceContent.RootFolder,
			[]string{"ClusterComputeResource"}, true)
		if err != nil {
			clusterCh <- clusterResult{err: err}
			return
		}
		defer cv.Destroy(ctx) //nolint:errcheck
		var moClusters []mo.ClusterComputeResource
		if err := cv.Retrieve(ctx, []string{"ClusterComputeResource"}, clusterProperties, &moClusters); err != nil {
			clusterCh <- clusterResult{err: err}
			return
		}
		clusters := make(map[string]string, len(moClusters))
		for _, c := range moClusters {
			clusters[c.Self.Value] = c.Name
		}
		clusterCh <- clusterResult{clusters: clusters}
	}()

	vmRes      := <-vmCh
	dsRes      := <-dsCh
	netRes     := <-netCh
	hostRes    := <-hostCh
	clusterRes := <-clusterCh

	if vmRes.err != nil {
		return nil, fmt.Errorf("vmware SyncInventory ListVMs: %w", vmRes.err)
	}
	if dsRes.err != nil {
		return nil, fmt.Errorf("vmware SyncInventory ListDataStores: %w", dsRes.err)
	}
	if netRes.err != nil {
		return nil, fmt.Errorf("vmware SyncInventory ListNetworks: %w", netRes.err)
	}
	// Host/cluster errors are non-fatal — log and continue without topology.
	if hostRes.err != nil {
		p.log.Warn("vmware: failed to fetch host topology (continuing without host names)",
			logger.Error(hostRes.err))
		hostRes.hosts = map[string]string{}
	}
	if clusterRes.err != nil {
		p.log.Warn("vmware: failed to fetch cluster topology (continuing without cluster names)",
			logger.Error(clusterRes.err))
		clusterRes.clusters = map[string]string{}
	}

	// Build datastore MOR→name map for VM enrichment.
	dsMORToName := make(map[string]string, len(dsRes.data))
	for _, ds := range dsRes.data {
		dsMORToName[ds.ProviderID] = ds.Name
	}

	// Build topology lookup.
	topo := &TopologyInfo{
		Hosts:      hostRes.hosts,
		Clusters:   clusterRes.clusters,
		Datastores: dsMORToName,
	}

	// Enrich each VM: resolve host→cluster by walking the host's parent chain.
	// HostSystem.Parent points to ComputeResource or ClusterComputeResource.
	// We need the host→cluster mapping, which requires fetching host parents.
	hostToCluster := p.buildHostClusterMap(ctx, hostRes.hosts, clusterRes.clusters)

	vmInfos := make([]port.VMInfo, 0, len(vmRes.vms))
	for i := range vmRes.vms {
		info := MapVMInfoWithTopology(&vmRes.vms[i], topo)
		// Enrich with cluster name via host→cluster map.
		if hostMOR, ok := info.Extra["esxi_host_mor"].(string); ok {
			if clusterName, ok := hostToCluster[hostMOR]; ok {
				info.Extra["cluster"] = clusterName
			}
		}
		vmInfos = append(vmInfos, info)
	}

	snap := &port.InventorySnapshot{
		VMs:        vmInfos,
		DataStores: dsRes.data,
		Networks:   netRes.data,
		SyncedAt:   timeNow(),
	}

	// Populate Hosts in the snapshot — use full mo.HostSystem for hardware details.
	moHostMap := make(map[string]mo.HostSystem, len(hostRes.moHosts))
	for _, h := range hostRes.moHosts {
		moHostMap[h.Self.Value] = h
	}
	for hostMOR, hostName := range hostRes.hosts {
		clusterName := hostToCluster[hostMOR]
		clusterProviderID := ""
		for clusterMOR, cName := range clusterRes.clusters {
			if cName == clusterName {
				clusterProviderID = clusterMOR
				break
			}
		}
		hi := port.HostInfo{
			ProviderID:        hostMOR,
			Name:              hostName,
			Status:            "connected",
			ClusterProviderID: clusterProviderID,
		}
		if moHost, ok := moHostMap[hostMOR]; ok {
			hw := moHost.Summary.Hardware
			rt := moHost.Summary.Runtime
			if hw != nil {
				hi.CPUModel   = hw.CpuModel
				hi.CPUSockets = int(hw.NumCpuPkgs)
				hi.CPUCores   = int(hw.NumCpuCores)
				hi.CPUThreads = int(hw.NumCpuThreads)
				hi.TotalMemoryMB = int(hw.MemorySize / (1024 * 1024))
			}
			if rt != nil {
				hi.HypervisorVersion = moHost.Summary.Config.Product.Version
				if rt.ConnectionState == "connected" {
					hi.Status = "connected"
				} else if rt.InMaintenanceMode {
					hi.Status = "maintenance"
				} else {
					hi.Status = "disconnected"
				}
				if !rt.BootTime.IsZero() {
					hi.UptimeSeconds = int64(time.Since(*rt.BootTime).Seconds())
				}
			}
			if qs := moHost.Summary.QuickStats; qs.OverallCpuUsage > 0 || qs.OverallMemoryUsage > 0 {
				hi.CPUUsageMHz  = int(qs.OverallCpuUsage)
				hi.UsedMemoryMB = int(qs.OverallMemoryUsage)
			}
		}
		snap.Hosts = append(snap.Hosts, hi)
	}

	// Populate Clusters in the snapshot
	for clusterMOR, clusterName := range clusterRes.clusters {
		hostCount := 0
		for _, cName := range hostToCluster {
			if cName == clusterName {
				hostCount++
			}
		}
		snap.Clusters = append(snap.Clusters, port.ClusterInfo{
			ProviderID: clusterMOR,
			Name:       clusterName,
			HostCount:  hostCount,
		})
	}

	p.log.Info("vmware: inventory sync complete",
		logger.Int("vms", len(snap.VMs)),
		logger.Int("datastores", len(snap.DataStores)),
		logger.Int("networks", len(snap.Networks)),
		logger.Int("hosts", len(hostRes.hosts)),
		logger.Int("clusters", len(clusterRes.clusters)),
	)
	return snap, nil
}

// buildHostClusterMap fetches the parent of each HostSystem and returns a
// map of hostMOR → clusterName for hosts that belong to a cluster.
// Hosts in standalone ComputeResource objects are skipped.
func (p *Provider) buildHostClusterMap(ctx context.Context, hosts map[string]string, clusters map[string]string) map[string]string {
	if len(hosts) == 0 || len(clusters) == 0 {
		return map[string]string{}
	}

	// Build refs for all known hosts.
	refs := make([]types.ManagedObjectReference, 0, len(hosts))
	for mor := range hosts {
		refs = append(refs, types.ManagedObjectReference{Type: "HostSystem", Value: mor})
	}

	pc := property.DefaultCollector(p.conn.client.Client)
	var moHosts []mo.HostSystem
	if err := pc.Retrieve(ctx, refs, []string{"parent"}, &moHosts); err != nil {
		p.log.Warn("vmware: failed to fetch host parents for cluster mapping", logger.Error(err))
		return map[string]string{}
	}

	result := make(map[string]string, len(moHosts))
	for _, h := range moHosts {
		if h.Parent == nil {
			continue
		}
		parentMOR := h.Parent.Value
		// Parent is ClusterComputeResource → host is in a cluster.
		if clusterName, ok := clusters[parentMOR]; ok {
			result[h.Self.Value] = clusterName
		}
	}
	return result
}

// ── Console Session ──────────────────────────────────────────────────────────

// GetConsoleSession acquires a WebMKS ticket from ESXi/vCenter for the given VM.
// Returns an HTTPS URL that opens the console directly in a browser.
//
// For standalone ESXi (6.5+): uses the ESXi embedded host client console URL
// at https://<host>/ui/#/console/<vmid> with the WebMKS ticket.
// For vCenter: uses the vCenter HTML5 client webconsole page.
func (p *Provider) GetConsoleSession(ctx context.Context, providerVMID string, opts port.ConsoleOptions) (*port.ConsoleSession, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	vm, err := p.findVM(ctx, providerVMID)
	if err != nil {
		return nil, err
	}

	// AcquireTicket issues a short-lived WebMKS ticket.
	ticket, err := vm.AcquireTicket(ctx, "webmks")
	if err != nil {
		return nil, provider.Wrap("vmware", "GetConsoleSession", provider.CodeInternal,
			"failed to acquire WebMKS ticket", err)
	}

	host := ticket.Host
	if host == "" {
		host = p.Credentials().Host
	}
	port_ := int(ticket.Port)
	if port_ == 0 {
		port_ = 443
	}

	ttl := consoleDefaultTTL(opts)

	// Raw WebMKS WebSocket URL — for embedding in a WebMKS-capable client.
	wssURL := fmt.Sprintf("wss://%s:%d/ticket/%s", host, port_, ticket.Ticket)

	// Browser-openable HTTPS URL.
	// ESXi 6.5+ embedded host client console endpoint — works on standalone ESXi
	// without vCenter. The host client accepts the WebMKS ticket via query param.
	var httpsURL string
	if port_ == 443 {
		httpsURL = fmt.Sprintf(
			"https://%s/ui/webconsole.html?vmId=%s&vmName=%s&host=%s&sessionTicket=%s&thumbprint=%s",
			host, providerVMID, providerVMID, host,
			url.QueryEscape(ticket.Ticket), url.QueryEscape(ticket.SslThumbprint),
		)
	} else {
		httpsURL = fmt.Sprintf(
			"https://%s:%d/ui/webconsole.html?vmId=%s&vmName=%s&host=%s&sessionTicket=%s&thumbprint=%s",
			host, port_, providerVMID, providerVMID, host,
			url.QueryEscape(ticket.Ticket), url.QueryEscape(ticket.SslThumbprint),
		)
	}

	session := &port.ConsoleSession{
		Type:      port.ConsoleTypeWebMKS,
		URL:       httpsURL,
		Ticket:    ticket.Ticket,
		Host:      host,
		Port:      port_,
		ExpiresAt: timeNow().Add(ttl),
		Extra: map[string]interface{}{
			"provider":       "vmware",
			"wss_url":        wssURL,
			"ssl_thumbprint": ticket.SslThumbprint,
			"cfg_file":       ticket.CfgFile,
		},
	}
	p.log.Info("vmware: console session acquired",
		logger.String("vm_id", providerVMID),
		logger.String("host", host),
		logger.String("url", httpsURL),
	)
	return session, nil
}

// ── Templates ────────────────────────────────────────────────────────────────

// ListTemplates returns all VM templates visible in vCenter.
// Templates are VirtualMachine objects with config.template = true.
func (p *Provider) ListTemplates(ctx context.Context) ([]port.TemplateInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	m := view.NewManager(p.conn.client.Client)
	cv, err := m.CreateContainerView(ctx, p.conn.client.ServiceContent.RootFolder,
		[]string{"VirtualMachine"}, true)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListTemplates", provider.CodeInternal,
			"failed to create container view", err)
	}
	defer cv.Destroy(ctx) //nolint:errcheck

	var moVMs []mo.VirtualMachine
	err = cv.Retrieve(ctx, []string{"VirtualMachine"},
		[]string{"config", "summary", "datastore"}, &moVMs)
	if err != nil {
		return nil, provider.Wrap("vmware", "ListTemplates", provider.CodeInternal,
			"failed to retrieve VM properties", err)
	}

	var infos []port.TemplateInfo
	for i := range moVMs {
		vm := &moVMs[i]
		if vm.Config == nil || !vm.Config.Template {
			continue // skip non-templates
		}
		info := port.TemplateInfo{
			ProviderID:  vm.Self.Value,
			Name:        vm.Config.Name,
			Description: vm.Config.Annotation,
			GuestOS:     vm.Config.GuestFullName,
			CPUCount:    int(vm.Config.Hardware.NumCPU),
			MemoryMB:    int(vm.Config.Hardware.MemoryMB),
			Extra: map[string]interface{}{
				"mor":      vm.Self.Value,
				"guest_id": vm.Config.GuestId,
			},
		}
		// Estimate disk size from the first disk device.
		for _, dev := range vm.Config.Hardware.Device {
			if disk, ok := dev.(*types.VirtualDisk); ok {
				info.DiskGB = int(disk.CapacityInKB / 1024 / 1024)
				break
			}
		}
		infos = append(infos, info)
	}

	p.log.Info("vmware: listed templates", logger.Int("count", len(infos)))
	return infos, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// findVM resolves a providerVMID (ManagedObject reference value like "vm-42")
// to a govmomi VirtualMachine object.
func (p *Provider) findVM(ctx context.Context, providerVMID string) (*object.VirtualMachine, error) {
	ref := types.ManagedObjectReference{Type: "VirtualMachine", Value: providerVMID}
	vm := object.NewVirtualMachine(p.conn.client.Client, ref)

	// Verify the object exists by fetching a minimal property.
	var moVM mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, ref, []string{"config.name"}, &moVM); err != nil {
		return nil, provider.Wrap("vmware", "findVM", provider.CodeNotFound,
			fmt.Sprintf("VM %q not found", providerVMID), err)
	}
	return vm, nil
}

// assertPowerState returns ErrInvalidState if the VM's current power state is
// not one of the allowed states.
func (p *Provider) assertPowerState(ctx context.Context, vm *object.VirtualMachine, allowed ...types.VirtualMachinePowerState) error {
	var moVM mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"runtime.powerState"}, &moVM); err != nil {
		// If we can't read the state, let the operation proceed and let vCenter reject it.
		return nil
	}
	for _, s := range allowed {
		if moVM.Runtime.PowerState == s {
			return nil
		}
	}
	return provider.New("vmware", "assertPowerState", provider.CodeInvalidState,
		fmt.Sprintf("VM is in state %q, operation not allowed", moVM.Runtime.PowerState))
}

// waitTask waits for a govmomi task to complete and maps errors to ProviderError.
func (p *Provider) waitTask(ctx context.Context, op, vmID string, task *object.Task) error {
	if err := task.Wait(ctx); err != nil {
		return provider.Wrap("vmware", op, provider.CodeInternal,
			fmt.Sprintf("task failed for VM %q", vmID), err)
	}
	p.log.Info("vmware: task completed",
		logger.String("operation", op),
		logger.String("vm_id", vmID),
	)
	return nil
}

// currentSnapshotRef returns the current snapshot MOR for a VM, or nil.
func (p *Provider) currentSnapshotRef(ctx context.Context, vm *object.VirtualMachine) *types.ManagedObjectReference {
	var moVM mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"snapshot"}, &moVM); err != nil {
		return nil
	}
	if moVM.Snapshot == nil {
		return nil
	}
	return moVM.Snapshot.CurrentSnapshot
}

// HostResourcePool resolves the resource pool for a given HostSystem MOR.
// Exported so the ESXi provider can reuse it.
// It walks: HostSystem → parent (ComputeResource or ClusterComputeResource)
// → resourcePool child, which is the implicit root pool for that host/cluster.
func (p *Provider) HostResourcePool(ctx context.Context, hostRef types.ManagedObjectReference) (types.ManagedObjectReference, error) {
	return p.hostResourcePool(ctx, hostRef)
}

// hostResourcePool is the unexported implementation.
func (p *Provider) hostResourcePool(ctx context.Context, hostRef types.ManagedObjectReference) (types.ManagedObjectReference, error) {
	// Fetch the host's parent (ComputeResource or ClusterComputeResource).
	var moHost mo.HostSystem
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, hostRef, []string{"parent"}, &moHost); err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("fetch host parent: %w", err)
	}
	if moHost.Parent == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("host %s has no parent", hostRef.Value)
	}

	// The parent is a ComputeResource or ClusterComputeResource.
	// Both have a "resourcePool" property pointing to the root resource pool.
	var moCompute mo.ComputeResource
	if err := pc.RetrieveOne(ctx, *moHost.Parent, []string{"resourcePool"}, &moCompute); err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("fetch compute resource pool: %w", err)
	}
	if moCompute.ResourcePool == nil {
		return types.ManagedObjectReference{}, fmt.Errorf("compute resource %s has no resource pool", moHost.Parent.Value)
	}
	return *moCompute.ResourcePool, nil
}

// vmFolder returns the parent folder of a VM. Falls back to the default folder.
func (p *Provider) vmFolder(ctx context.Context, vm *object.VirtualMachine, finder *find.Finder) (*object.Folder, error) {
	var moVM mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"parent"}, &moVM); err == nil && moVM.Parent != nil {
		if moVM.Parent.Type == "Folder" {
			return object.NewFolder(p.conn.client.Client, *moVM.Parent), nil
		}
	}
	return finder.DefaultFolder(ctx)
}

// vmResourcePool returns the resource pool of a VM's host/cluster.
// For a VM already assigned to a pool, that pool is returned directly.
// For a template (which has no pool), the default resource pool is used.
func (p *Provider) vmResourcePool(ctx context.Context, vm *object.VirtualMachine, finder *find.Finder) (*object.ResourcePool, error) {
	var moVM mo.VirtualMachine
	pc := property.DefaultCollector(p.conn.client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"resourcePool"}, &moVM); err == nil && moVM.ResourcePool != nil {
		return object.NewResourcePool(p.conn.client.Client, *moVM.ResourcePool), nil
	}
	// Templates have no resource pool — fall back to the default.
	return finder.DefaultResourcePool(ctx)
}

// cloneFromTemplate deploys a new VM by cloning a template.
func (p *Provider) cloneFromTemplate(ctx context.Context, finder *find.Finder, spec port.VMCreateSpec) (*port.VMInfo, error) {
	tmpl, err := finder.VirtualMachine(ctx, spec.TemplateID)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeNotFound,
			fmt.Sprintf("template %q not found", spec.TemplateID), err)
	}

	folder, err := finder.DefaultFolder(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"no default folder", err)
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		PowerOn:  false,
		Template: false,
		Config: &types.VirtualMachineConfigSpec{
			NumCPUs:    int32(spec.CPUCount),
			MemoryMB:   int64(spec.MemoryMB),
			Annotation: "",
		},
	}

	if spec.DataStore != "" {
		ds, err := finder.Datastore(ctx, spec.DataStore)
		if err == nil {
			ref := ds.Reference()
			cloneSpec.Location.Datastore = &ref
		}
	}

	task, err := tmpl.Clone(ctx, folder, spec.Name, cloneSpec)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"template clone failed", err)
	}
	result, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"template clone task failed", err)
	}

	newVM := object.NewVirtualMachine(p.conn.client.Client, result.Result.(types.ManagedObjectReference))
	return p.GetVM(ctx, newVM.Reference().Value)
}

// createBlankVM creates a minimal VM configuration without a template.
func (p *Provider) createBlankVM(ctx context.Context, finder *find.Finder, spec port.VMCreateSpec) (*port.VMInfo, error) {
	folder, err := finder.DefaultFolder(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"no default folder", err)
	}

	pool, err := finder.DefaultResourcePool(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"no default resource pool", err)
	}

	vmSpec := types.VirtualMachineConfigSpec{
		Name:     spec.Name,
		NumCPUs:  int32(spec.CPUCount),
		MemoryMB: int64(spec.MemoryMB),
		GuestId:  spec.GuestOS,
		Files:    &types.VirtualMachineFileInfo{VmPathName: fmt.Sprintf("[%s]", spec.DataStore)},
	}

	task, err := folder.CreateVM(ctx, vmSpec, pool, nil)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"VM creation failed", err)
	}
	result, err := task.WaitForResult(ctx)
	if err != nil {
		return nil, provider.Wrap("vmware", "CreateVM", provider.CodeInternal,
			"VM creation task failed", err)
	}

	newVM := object.NewVirtualMachine(p.conn.client.Client, result.Result.(types.ManagedObjectReference))
	return p.GetVM(ctx, newVM.Reference().Value)
}
