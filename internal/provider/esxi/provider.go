// Package esxi implements port.Provider for standalone VMware ESXi hosts.
//
// A standalone ESXi host exposes the same govmomi/SOAP SDK endpoint as vCenter
// (/sdk on port 443), so this provider reuses the VMware client and mapper
// entirely. The key differences from vCenter are:
//
//   - No ClusterComputeResource objects (no clusters on standalone ESXi)
//   - No DistributedVirtualPortgroup (only standard port groups)
//   - No vCenter Performance Manager (GetVMMetrics returns empty metrics)
//   - The ESXi host itself is the only HostSystem
//
// SyncInventory is overridden to handle these differences gracefully instead
// of failing when cluster/DVS queries return errors on standalone ESXi.
package esxi

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/internal/provider/vmware"
	"github.com/vmOrbit/backend/pkg/logger"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Provider implements port.Provider for a standalone ESXi host.
// It embeds the VMware provider to reuse all govmomi client logic,
// connection management, VM operations, snapshots, and power actions.
// Only Type(), Name(), Capabilities(), Connect(), and SyncInventory() are overridden.
type Provider struct {
	*vmware.Provider
	log logger.Logger
}

// NewProvider creates a new ESXi provider instance.
// It uses the same VMware config and govmomi client under the hood.
func NewProvider(cfg config.VMwareConfig, log logger.Logger) *Provider {
	return &Provider{
		Provider: vmware.NewProvider(cfg, log),
		log:      log,
	}
}

// ── Meta ─────────────────────────────────────────────────────────────────────

// Type returns the ESXi provider type, distinguishing it from vCenter.
func (p *Provider) Type() model.ProviderType { return model.ProviderESXi }

// Name returns a human-readable name.
func (p *Provider) Name() string { return "VMware ESXi" }

// Capabilities declares the ESXi feature set.
// Standalone ESXi does not have a Performance Manager (no GuestMetrics),
// and does not support vMotion (no LiveMigration).
func (p *Provider) Capabilities() port.ProviderCapabilities {
	return port.ProviderCapabilities{
		Console:          true,  // WebMKS via AcquireTicket — works on standalone ESXi
		LinkedClones:     false, // requires vCenter
		MemorySnapshots:  true,  // ESXi supports memory snapshots
		QuiesceSnapshots: true,  // requires VMware Tools in guest
		LiveMigration:    false, // vMotion requires vCenter
		GuestMetrics:     false, // Performance Manager not available on standalone ESXi
		TemplateClone:    false, // template deployment requires vCenter
	}
}

// Connect logs the ESXi-specific connection attempt before delegating to the
// embedded VMware provider which handles the govmomi dial and authentication.
func (p *Provider) Connect(ctx context.Context, creds port.Credentials) error {
	p.log.Info("esxi: connecting to standalone ESXi host",
		logger.String("host", creds.Host),
		logger.Int("port", creds.Port),
		logger.Bool("tls_verify", creds.TLSVerify),
	)

	if err := p.Provider.Connect(ctx, creds); err != nil {
		p.log.Error("esxi: connection failed",
			logger.String("host", creds.Host),
			logger.Error(err),
		)
		return err
	}

	p.log.Info("esxi: authentication successful — connected to ESXi host",
		logger.String("host", creds.Host),
	)
	return nil
}

// ── Console Session ───────────────────────────────────────────────────────────

// GetConsoleSession acquires an MKS ticket from a standalone ESXi host.
//
// Standalone ESXi does NOT support the "webmks" ticket type — that is a
// vCenter-only feature. ESXi only supports "mks" (the legacy VMRC protocol).
// We acquire an "mks" ticket and build a direct wss:// URL that the browser
// can use to connect via the ESXi host client's built-in WebMKS bridge.
func (p *Provider) GetConsoleSession(ctx context.Context, providerVMID string, opts port.ConsoleOptions) (*port.ConsoleSession, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	govmomiClient := p.Client()
	if govmomiClient == nil {
		return nil, provider.New("esxi", "GetConsoleSession", provider.CodeNotConnected,
			"govmomi client is nil — call Connect first")
	}

	// Standalone ESXi only supports "mks" ticket type.
	// "webmks" is vCenter-only and returns:
	//   ServerFaultCode: A specified parameter was not correct. ticketType
	vmRef := types.ManagedObjectReference{Type: "VirtualMachine", Value: providerVMID}
	vmObj := object.NewVirtualMachine(govmomiClient.Client, vmRef)

	ticket, err := vmObj.AcquireTicket(ctx, "mks")
	if err != nil {
		return nil, provider.Wrap("esxi", "GetConsoleSession", provider.CodeInternal,
			"failed to acquire MKS ticket", err)
	}

	esxiHost := ticket.Host
	if esxiHost == "" {
		esxiHost = p.Credentials().Host
	}
	port_ := int(ticket.Port)
	if port_ == 0 {
		port_ = 443
	}

	ttl := consoleDefaultTTL(opts)

	// Port 902 is the ESXi MKS port — it uses plain WebSocket (no TLS).
	// Port 443 is the ESXi host client HTTPS port — uses TLS.
	wsScheme := "wss"
	if port_ == 902 {
		wsScheme = "ws"
	}

	// Direct WebSocket URL for the browser to connect to.
	wsURL := fmt.Sprintf("%s://%s:%d/ticket/%s", wsScheme, esxiHost, port_, ticket.Ticket)

	// Browser-openable HTTPS URL — opens the ESXi host client console page.
	httpsURL := fmt.Sprintf(
		"https://%s/ui/webconsole.html?vmId=%s&vmName=%s&host=%s&sessionTicket=%s&thumbprint=%s",
		esxiHost, providerVMID, providerVMID, esxiHost,
		url.QueryEscape(ticket.Ticket), url.QueryEscape(ticket.SslThumbprint),
	)

	session := &port.ConsoleSession{
		Type:      port.ConsoleTypeWebMKS,
		URL:       httpsURL,
		Ticket:    ticket.Ticket,
		Host:      esxiHost,
		Port:      port_,
		ExpiresAt: timeNow().Add(ttl),
		Extra: map[string]interface{}{
			"provider":       "esxi",
			"wss_url":        wsURL,
			"mks_port":       port_,
			"ws_scheme":      wsScheme,
			"ssl_thumbprint": ticket.SslThumbprint,
			"cfg_file":       ticket.CfgFile,
		},
	}

	p.log.Info("esxi: console session acquired",
		logger.String("vm_id", providerVMID),
		logger.String("host", esxiHost),
		logger.Int("port", port_),
		logger.String("cfg_file", ticket.CfgFile),
		logger.String("ticket_prefix", ticket.Ticket[:min(8, len(ticket.Ticket))]),
	)
	return session, nil
}

// CloneVM implements VM cloning on standalone ESXi using disk copy + RegisterVM.
//
// ESXi does not support the vCenter CloneVM_Task API, so we implement cloning
// manually in three steps:
//  1. Read the source VM's hardware config (disks, vmx path, CPU, memory).
//  2. Copy each virtual disk to a new directory on the same datastore using
//     VirtualDiskManager.CopyVirtualDisk (preserves disk format correctly).
//  3. Register a new VM from a freshly-built config spec pointing at the
//     copied disks, using Folder.RegisterVM.
//
// Limitations vs vCenter CloneVM:
//   - Full clone only (linked clones require vCenter delta-disk support).
//   - The clone lands on the same datastore as the source unless spec.DataStore
//     is set to a different datastore name accessible on this ESXi host.
//   - The source VM must be powered off or have no locked disks.
func (p *Provider) CloneVM(ctx context.Context, providerVMID string, spec port.VMCloneSpec) (*port.VMInfo, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	if spec.Linked {
		return nil, provider.New("esxi", "CloneVM", provider.CodeUnsupported,
			"linked clones require vCenter — standalone ESXi supports full clones only")
	}

	client := p.Client()
	if client == nil {
		return nil, provider.New("esxi", "CloneVM", provider.CodeNotConnected,
			"govmomi client is nil")
	}
	vim := client.Client

	// ── 1. Fetch source VM properties ────────────────────────────────────────
	srcRef := types.ManagedObjectReference{Type: "VirtualMachine", Value: providerVMID}
	var moSrc mo.VirtualMachine
	pc := property.DefaultCollector(vim)
	if err := pc.RetrieveOne(ctx, srcRef, []string{
		"config", "runtime.host", "datastore", "parent",
	}, &moSrc); err != nil {
		return nil, provider.Wrap("esxi", "CloneVM", provider.CodeNotFound,
			fmt.Sprintf("source VM %q not found", providerVMID), err)
	}
	if moSrc.Config == nil {
		return nil, provider.New("esxi", "CloneVM", provider.CodeInternal,
			"source VM has no config — cannot clone")
	}

	// ── 2. Resolve datastore and host ─────────────────────────────────────────
	// Determine the target datastore name. Default to the source VM's datastore.
	srcDSName, _, err := p.resolveSourceDatastore(ctx, vim, moSrc)
	if err != nil {
		return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
			"could not resolve source datastore", err)
	}

	dstDSName := srcDSName
	if spec.DataStore != "" && spec.DataStore != srcDSName {
		// Caller requested a different datastore — look it up.
		dsInfo, dsErr := p.Provider.GetDataStore(ctx, spec.DataStore)
		if dsErr != nil {
			p.log.Warn("esxi: CloneVM target datastore not found, using source datastore",
				logger.String("requested", spec.DataStore),
				logger.String("fallback", srcDSName),
			)
		} else {
			dstDSName = dsInfo.Name
		}
	}

	// Resolve the ESXi host MOR for RegisterVM.
	if moSrc.Runtime.Host == nil {
		return nil, provider.New("esxi", "CloneVM", provider.CodeInternal,
			"source VM has no runtime host — cannot determine target host")
	}
	hostRef := *moSrc.Runtime.Host

	// ── 3. Build destination paths ────────────────────────────────────────────
	// VMX path format: [datastoreName] vmName/vmName.vmx
	dstDir := fmt.Sprintf("[%s] %s", dstDSName, spec.Name)

	// ── 4. Copy virtual disks ─────────────────────────────────────────────────
	diskMgr := object.NewVirtualDiskManager(vim)
	// On standalone ESXi there is only one datacenter — pass nil to use it implicitly.
	// CopyVirtualDisk and DeleteVirtualDisk accept nil *object.Datacenter.

	// Collect disk backing paths from the source config.
	type diskCopy = diskCopyPair
	var diskCopies []diskCopy

	for _, dev := range moSrc.Config.Hardware.Device {
		disk, ok := dev.(*types.VirtualDisk)
		if !ok {
			continue
		}
		backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
		if !ok {
			continue
		}
		srcPath := backing.FileName // e.g. "[datastore1] srcVM/srcVM.vmdk"
		// Build destination path: [dstDS] cloneName/disk-N.vmdk
		diskName := fmt.Sprintf("%s/%s-%d.vmdk", dstDir, spec.Name, disk.Key)
		diskCopies = append(diskCopies, diskCopy{srcPath: srcPath, dstPath: diskName})
	}

	if len(diskCopies) == 0 {
		return nil, provider.New("esxi", "CloneVM", provider.CodeInternal,
			"source VM has no clonable virtual disks")
	}

	p.log.Info("esxi: cloning VM via disk copy",
		logger.String("src_vm_id", providerVMID),
		logger.String("name", spec.Name),
		logger.String("dst_datastore", dstDSName),
		logger.Int("disk_count", len(diskCopies)),
	)

	// Copy each disk. CopyVirtualDisk preserves the disk format (thin/thick).
	copiedDisks := make([]string, 0, len(diskCopies))
	for _, dc := range diskCopies {
		p.log.Info("esxi: copying disk",
			logger.String("src", dc.srcPath),
			logger.String("dst", dc.dstPath),
		)
		copyTask, err := diskMgr.CopyVirtualDisk(ctx,
			dc.srcPath, nil, // nil datacenter = use default (only one on ESXi)
			dc.dstPath, nil,
			nil,   // keep same disk spec
			false, // don't force
		)
		if err != nil {
			p.cleanupDisks(ctx, diskMgr, copiedDisks)
			return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
				fmt.Sprintf("disk copy failed for %s", dc.srcPath), err)
		}
		if err := copyTask.Wait(ctx); err != nil {
			p.cleanupDisks(ctx, diskMgr, copiedDisks)
			return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
				fmt.Sprintf("disk copy task failed for %s", dc.srcPath), err)
		}
		copiedDisks = append(copiedDisks, dc.dstPath)
	}

	// ── 5. Build new VM config spec with copied disks ─────────────────────────
	newConfig := p.buildCloneConfigSpec(moSrc.Config, spec.Name, dstDSName, diskCopies)

	// ── 6. Register the new VM ────────────────────────────────────────────────
	// Find the VM folder — use the source VM's parent folder.
	var vmFolder *object.Folder
	if moSrc.Parent != nil && moSrc.Parent.Type == "Folder" {
		vmFolder = object.NewFolder(vim, *moSrc.Parent)
	} else {
		finder := find.NewFinder(client.Client, true)
		dc, dcErr := finder.DefaultDatacenter(ctx)
		if dcErr != nil {
			p.cleanupDisks(ctx, diskMgr, copiedDisks)
			return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
				"could not find datacenter for VM registration", dcErr)
		}
		finder.SetDatacenter(dc)
		vmFolder, err = finder.DefaultFolder(ctx)
		if err != nil {
			p.cleanupDisks(ctx, diskMgr, copiedDisks)
			return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
				"could not find VM folder for registration", err)
		}
	}

	// Resolve resource pool from the host.
	hostPool, poolErr := p.Provider.HostResourcePool(ctx, hostRef)
	if poolErr != nil {
		p.cleanupDisks(ctx, diskMgr, copiedDisks)
		return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
			"could not resolve resource pool", poolErr)
	}
	pool := object.NewResourcePool(vim, hostPool)
	host := object.NewHostSystem(vim, hostRef)

	createTask, err := vmFolder.CreateVM(ctx, newConfig, pool, host)
	if err != nil {
		p.cleanupDisks(ctx, diskMgr, copiedDisks)
		return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
			"VM registration failed", err)
	}
	result, err := createTask.WaitForResult(ctx)
	if err != nil {
		p.cleanupDisks(ctx, diskMgr, copiedDisks)
		return nil, provider.Wrap("esxi", "CloneVM", provider.CodeInternal,
			"VM registration task failed", err)
	}

	newVMRef := result.Result.(types.ManagedObjectReference)
	p.log.Info("esxi: VM cloned successfully",
		logger.String("src_vm_id", providerVMID),
		logger.String("new_vm_id", newVMRef.Value),
		logger.String("name", spec.Name),
	)

	return p.Provider.GetVM(ctx, newVMRef.Value)
}

// SyncInventory fetches the full live inventory from a standalone ESXi host.
//
// Unlike the vCenter implementation, this version:
//   - Skips ClusterComputeResource queries (no clusters on standalone ESXi)
//   - Treats the single HostSystem as the ESXi host for all VMs
//   - Handles missing DVS/distributed port groups gracefully
//   - Enriches each VM with the ESXi hostname from the HostSystem
//
// VMs, datastores, networks, and the host are fetched concurrently.
func (p *Provider) SyncInventory(ctx context.Context) (*port.InventorySnapshot, error) {
	if err := p.EnsureConnected(ctx); err != nil {
		return nil, err
	}

	p.log.Info("esxi: starting inventory sync")

	// Access the underlying govmomi client via the embedded provider.
	govmomiClient := p.Client()
	if govmomiClient == nil {
		return nil, provider.New("esxi", "SyncInventory", provider.CodeNotConnected,
			"govmomi client is nil — call Connect first")
	}

	type vmResult struct {
		vms []mo.VirtualMachine
		err error
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
		hosts   map[string]string // MOR → hostname (for VM topology enrichment)
		moHosts []mo.HostSystem   // full objects for hardware mapping
		err     error
	}

	vmCh   := make(chan vmResult, 1)
	dsCh   := make(chan dsResult, 1)
	netCh  := make(chan netResult, 1)
	hostCh := make(chan hostResult, 1)

	m := view.NewManager(govmomiClient.Client)
	rootFolder := govmomiClient.ServiceContent.RootFolder

	// ── Goroutine 1: VMs ─────────────────────────────────────────────────────
	go func() {
		p.log.Debug("esxi: fetching VM inventory via ContainerView")
		cv, err := m.CreateContainerView(ctx, rootFolder, []string{"VirtualMachine"}, true)
		if err != nil {
			vmCh <- vmResult{err: fmt.Errorf("esxi: create VM container view: %w", err)}
			return
		}
		defer cv.Destroy(ctx) //nolint:errcheck

		var moVMs []mo.VirtualMachine
		if err := cv.Retrieve(ctx, []string{"VirtualMachine"}, vmware.VMProperties, &moVMs); err != nil {
			vmCh <- vmResult{err: fmt.Errorf("esxi: retrieve VM properties: %w", err)}
			return
		}
		p.log.Info("esxi: VM discovery complete", logger.Int("count", len(moVMs)))
		vmCh <- vmResult{vms: moVMs}
	}()

	// ── Goroutine 2: Datastores ───────────────────────────────────────────────
	go func() {
		p.log.Debug("esxi: fetching datastore inventory")
		ds, err := p.Provider.ListDataStores(ctx)
		if err != nil {
			p.log.Warn("esxi: datastore fetch failed (continuing without datastores)",
				logger.Error(err))
			dsCh <- dsResult{data: []port.DataStoreInfo{}}
			return
		}
		p.log.Info("esxi: datastore discovery complete", logger.Int("count", len(ds)))
		dsCh <- dsResult{data: ds}
	}()

	// ── Goroutine 3: Networks (standard port groups only) ─────────────────────
	go func() {
		p.log.Debug("esxi: fetching network inventory")
		nets, err := p.Provider.ListNetworks(ctx)
		if err != nil {
			p.log.Warn("esxi: network fetch failed (continuing without networks)",
				logger.Error(err))
			netCh <- netResult{data: []port.NetworkInfo{}}
			return
		}
		p.log.Info("esxi: network discovery complete", logger.Int("count", len(nets)))
		netCh <- netResult{data: nets}
	}()

	// ── Goroutine 4: ESXi host (for hostname enrichment) ─────────────────────
	go func() {
		p.log.Debug("esxi: fetching host system info for VM enrichment")
		cv, err := m.CreateContainerView(ctx, rootFolder, []string{"HostSystem"}, true)
		if err != nil {
			p.log.Warn("esxi: host container view failed (continuing without host names)",
				logger.Error(err))
			hostCh <- hostResult{hosts: map[string]string{}}
			return
		}
		defer cv.Destroy(ctx) //nolint:errcheck

		var moHosts []mo.HostSystem
		if err := cv.Retrieve(ctx, []string{"HostSystem"},
			[]string{"name", "summary.hardware", "summary.runtime", "summary.quickStats", "summary.config.product"}, &moHosts); err != nil {
			p.log.Warn("esxi: host property retrieval failed (continuing without host names)",
				logger.Error(err))
			hostCh <- hostResult{hosts: map[string]string{}}
			return
		}

		hosts := make(map[string]string, len(moHosts))
		for _, h := range moHosts {
			hosts[h.Self.Value] = h.Name
			p.log.Info("esxi: discovered ESXi host",
				logger.String("host_mor", h.Self.Value),
				logger.String("hostname", h.Name),
			)
		}
		hostCh <- hostResult{hosts: hosts, moHosts: moHosts}
	}()

	// ── Collect results ───────────────────────────────────────────────────────
	vmRes   := <-vmCh
	dsRes   := <-dsCh
	netRes  := <-netCh
	hostRes := <-hostCh

	// VM fetch is fatal — we can't sync without VMs.
	if vmRes.err != nil {
		p.log.Error("esxi: VM inventory fetch failed", logger.Error(vmRes.err))
		return nil, vmRes.err
	}

	p.log.Info("esxi: all inventory goroutines complete",
		logger.Int("vms", len(vmRes.vms)),
		logger.Int("datastores", len(dsRes.data)),
		logger.Int("networks", len(netRes.data)),
		logger.Int("hosts", len(hostRes.hosts)),
	)

	// Build datastore MOR→name map for VM enrichment.
	dsMORToName := make(map[string]string, len(dsRes.data))
	for _, ds := range dsRes.data {
		dsMORToName[ds.ProviderID] = ds.Name
	}

	// Build topology info — no clusters on standalone ESXi.
	topo := &vmware.TopologyInfo{
		Hosts:      hostRes.hosts,
		Clusters:   map[string]string{}, // always empty on standalone ESXi
		Datastores: dsMORToName,
	}

	// Map VMs with topology enrichment (host name, datastore name).
	vmInfos := make([]port.VMInfo, 0, len(vmRes.vms))
	for i := range vmRes.vms {
		info := vmware.MapVMInfoWithTopology(&vmRes.vms[i], topo)
		vmInfos = append(vmInfos, info)
	}

	snap := &port.InventorySnapshot{
		VMs:        vmInfos,
		DataStores: dsRes.data,
		Networks:   netRes.data,
		SyncedAt:   timeNow(),
	}

	// Populate Hosts with full hardware details from mo.HostSystem.
	for _, moHost := range hostRes.moHosts {
		hi := port.HostInfo{
			ProviderID: moHost.Self.Value,
			Name:       moHost.Name,
			Status:     "connected",
		}
		if hw := moHost.Summary.Hardware; hw != nil {
			hi.CPUModel     = hw.CpuModel
			hi.CPUSockets   = int(hw.NumCpuPkgs)
			hi.CPUCores     = int(hw.NumCpuCores)
			hi.CPUThreads   = int(hw.NumCpuThreads)
			hi.TotalMemoryMB = int(hw.MemorySize / (1024 * 1024))
		}
		if rt := moHost.Summary.Runtime; rt != nil {
			if rt.ConnectionState == "connected" {
				hi.Status = "connected"
			} else if rt.InMaintenanceMode {
				hi.Status = "maintenance"
			} else {
				hi.Status = "disconnected"
			}
			if rt.BootTime != nil && !rt.BootTime.IsZero() {
				hi.UptimeSeconds = int64(time.Since(*rt.BootTime).Seconds())
			}
		}
		if cfg := moHost.Summary.Config; cfg.Product.Version != "" {
			hi.HypervisorVersion = cfg.Product.Version
		}
		qs := moHost.Summary.QuickStats
		if qs.OverallCpuUsage > 0 || qs.OverallMemoryUsage > 0 {
			hi.CPUUsageMHz  = int(qs.OverallCpuUsage)
			hi.UsedMemoryMB = int(qs.OverallMemoryUsage)
		}
		snap.Hosts = append(snap.Hosts, hi)
	}

	p.log.Info("esxi: inventory sync complete — saving to database",
		logger.Int("vms", len(snap.VMs)),
		logger.Int("datastores", len(snap.DataStores)),
		logger.Int("networks", len(snap.Networks)),
		logger.Int("hosts", len(snap.Hosts)),
	)
	return snap, nil
}

// diskCopyPair holds source and destination paths for a virtual disk copy.
type diskCopyPair struct {
	srcPath string
	dstPath string
}

// resolveSourceDatastore returns the name and MOR value of the primary
// datastore for the given VM (first entry in vm.Datastore).
func (p *Provider) resolveSourceDatastore(ctx context.Context, vim *vim25.Client, moVM mo.VirtualMachine) (name, morValue string, err error) {
	if len(moVM.Datastore) == 0 {
		return "", "", fmt.Errorf("VM has no datastores")
	}
	dsMOR := moVM.Datastore[0]
	var moDS mo.Datastore
	pc := property.DefaultCollector(vim)
	if err := pc.RetrieveOne(ctx, dsMOR, []string{"summary.name"}, &moDS); err != nil {
		return "", "", fmt.Errorf("fetch datastore name: %w", err)
	}
	return moDS.Summary.Name, dsMOR.Value, nil
}

// buildCloneConfigSpec constructs a VirtualMachineConfigSpec for the new VM,
// reusing the source hardware config but replacing disk backing paths with
// the copied disk paths and setting the new VM name.
func (p *Provider) buildCloneConfigSpec(
	srcCfg *types.VirtualMachineConfigInfo,
	name, dstDSName string,
	diskCopies []diskCopyPair,
) types.VirtualMachineConfigSpec {
	// Build a path→dstPath lookup for quick replacement.
	diskMap := make(map[string]string, len(diskCopies))
	for _, dc := range diskCopies {
		diskMap[dc.srcPath] = dc.dstPath
	}

	// Clone the device list, replacing disk backing paths.
	devices := make([]types.BaseVirtualDevice, 0, len(srcCfg.Hardware.Device))
	for _, dev := range srcCfg.Hardware.Device {
		disk, ok := dev.(*types.VirtualDisk)
		if !ok {
			devices = append(devices, dev)
			continue
		}
		backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
		if !ok {
			devices = append(devices, dev)
			continue
		}
		// Deep-copy the disk and update the backing filename.
		newBacking := *backing
		if dst, found := diskMap[backing.FileName]; found {
			newBacking.FileName = dst
		}
		newBacking.Parent = nil // clear parent chain — this is a full clone
		newDisk := *disk
		newDisk.Backing = &newBacking
		devices = append(devices, &newDisk)
	}

	spec := types.VirtualMachineConfigSpec{
		Name:     name,
		NumCPUs:  srcCfg.Hardware.NumCPU,
		MemoryMB: int64(srcCfg.Hardware.MemoryMB),
		GuestId:  srcCfg.GuestId,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: fmt.Sprintf("[%s] %s/%s.vmx", dstDSName, name, name),
		},
		DeviceChange: make([]types.BaseVirtualDeviceConfigSpec, 0, len(devices)),
	}

	for _, dev := range devices {
		spec.DeviceChange = append(spec.DeviceChange, &types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
			Device:    dev,
		})
	}

	return spec
}

// cleanupDisks deletes already-copied disks when a clone operation fails
// partway through, to avoid leaving orphaned vmdk files on the datastore.
func (p *Provider) cleanupDisks(ctx context.Context, diskMgr *object.VirtualDiskManager, paths []string) {
	for _, path := range paths {
		task, err := diskMgr.DeleteVirtualDisk(ctx, path, nil) // nil = default datacenter
		if err != nil {
			p.log.Warn("esxi: cleanup: failed to delete orphaned disk",
				logger.String("path", path), logger.Error(err))
			continue
		}
		if err := task.Wait(ctx); err != nil {
			p.log.Warn("esxi: cleanup: delete disk task failed",
				logger.String("path", path), logger.Error(err))
		}
	}
}
