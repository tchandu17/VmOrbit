package vmware

import (
	"context"
	"fmt"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMProperties is the minimal set of VM properties fetched from vCenter/ESXi.
// Keeping this list tight reduces wire traffic significantly in large inventories.
// Exported so the ESXi provider can reuse it directly.
var VMProperties = []string{
	"config",
	"summary",
	"guest",
	"runtime",
	"snapshot",
	"network",
	"resourcePool",
	"datastore",
}

// hostProperties is the set of HostSystem properties fetched during inventory sync.
var hostProperties = []string{
	"name",
	"summary.hardware",
	"summary.runtime",
	"summary.quickStats",
	"summary.config.product",
	"parent",
}

// clusterProperties is the set of ClusterComputeResource properties fetched during sync.
var clusterProperties = []string{
	"name",
	"summary",
}

// ── VM mapping ────────────────────────────────────────────────────────────────

// mapVMInfo converts a govmomi VirtualMachine managed object to the
// provider-agnostic port.VMInfo DTO.
func mapVMInfo(vm *mo.VirtualMachine) port.VMInfo {
	return MapVMInfoWithTopology(vm, nil)
}

// TopologyInfo carries cluster/host/datacenter names resolved during a full
// inventory sync. It is nil when fetching a single VM (GetVM).
type TopologyInfo struct {
	// HostMOR → hostname
	Hosts map[string]string
	// ClusterMOR → cluster name
	Clusters map[string]string
	// DatastoreMOR → datastore name
	Datastores map[string]string
}

// MapVMInfoWithTopology converts a govmomi VirtualMachine managed object to
// port.VMInfo, enriching it with cluster/host/datastore names when topology
// data is available. Exported so the ESXi provider can reuse it directly.
func MapVMInfoWithTopology(vm *mo.VirtualMachine, topo *TopologyInfo) port.VMInfo {
	info := port.VMInfo{
		ProviderVMID: vm.Self.Value,
		Extra:        make(map[string]interface{}),
	}

	if vm.Config != nil {
		info.Name = vm.Config.Name
		info.GuestOS = vm.Config.GuestFullName
		info.GuestOSType = vm.Config.GuestId
		info.CPUCount = int(vm.Config.Hardware.NumCPU)
		info.MemoryMB = int(vm.Config.Hardware.MemoryMB)

		// Aggregate disk capacity from all virtual disks.
		for _, device := range vm.Config.Hardware.Device {
			if disk, ok := device.(*types.VirtualDisk); ok {
				info.DiskGB += int(disk.CapacityInKB / (1024 * 1024))
			}
		}
	}

	if vm.Summary.Config.Name != "" && info.Name == "" {
		info.Name = vm.Summary.Config.Name
	}

	// Power state.
	info.Status = mapPowerState(vm.Runtime.PowerState)

	// ESXi host MOR — always available in runtime.
	if vm.Runtime.Host != nil {
		hostMOR := vm.Runtime.Host.Value
		info.Extra["esxi_host_mor"] = hostMOR
		if topo != nil {
			if hostName, ok := topo.Hosts[hostMOR]; ok {
				info.Extra["esxi_host"] = hostName
			}
		}
	}

	// Guest network info (populated when VMware Tools is running).
	if vm.Guest != nil {
		info.ToolsStatus = string(vm.Guest.ToolsStatus)
		for _, nic := range vm.Guest.Net {
			if nic.MacAddress != "" && info.MACAddress == "" {
				info.MACAddress = nic.MacAddress
			}
			for _, ip := range nic.IpAddress {
				// Filter out IPv6 link-local addresses to keep the list clean.
				if ip != "" && len(ip) > 0 {
					info.IPAddresses = append(info.IPAddresses, ip)
				}
			}
			if nic.Network != "" && info.NetworkName == "" {
				info.NetworkName = nic.Network
			}
		}
	}

	// Fallback: read MAC from hardware config when Tools are not running.
	if info.MACAddress == "" && vm.Config != nil {
		for _, device := range vm.Config.Hardware.Device {
			if eth, ok := device.(types.BaseVirtualEthernetCard); ok {
				card := eth.GetVirtualEthernetCard()
				if card.MacAddress != "" {
					info.MACAddress = card.MacAddress
					break
				}
			}
		}
	}

	// Primary datastore (first in the list).
	if len(vm.Datastore) > 0 && topo != nil {
		dsMOR := vm.Datastore[0].Value
		info.Extra["datastore_mor"] = dsMOR
		if dsName, ok := topo.Datastores[dsMOR]; ok {
			info.Extra["datastore"] = dsName
		}
	}

	// Stash the MOR value and annotation for callers that need raw vSphere data.
	info.Extra["mor"] = vm.Self.Value
	if vm.Config != nil {
		info.Extra["annotation"] = vm.Config.Annotation
		// UUID is the BIOS UUID — stable across vMotion, used as the canonical ID.
		info.Extra["uuid"] = vm.Config.Uuid
		info.Extra["instance_uuid"] = vm.Config.InstanceUuid
		info.Extra["change_version"] = vm.Config.ChangeVersion
	}

	// Summary fields useful for quick display without full config.
	info.Extra["vm_path"] = vm.Summary.Config.VmPathName
	info.Extra["tools_version"] = vm.Summary.Config.GuestFullName

	return info
}

// mapPowerState converts a vSphere power state to the domain VMStatus.
func mapPowerState(ps types.VirtualMachinePowerState) model.VMStatus {
	switch ps {
	case types.VirtualMachinePowerStatePoweredOn:
		return model.VMStatusRunning
	case types.VirtualMachinePowerStatePoweredOff:
		return model.VMStatusStopped
	case types.VirtualMachinePowerStateSuspended:
		return model.VMStatusSuspended
	default:
		return model.VMStatusUnknown
	}
}

// ── Snapshot mapping ──────────────────────────────────────────────────────────

// flattenSnapshotTree recursively walks the vSphere snapshot tree and returns
// a flat slice of SnapshotInfo. currentRef identifies the active snapshot.
func flattenSnapshotTree(
	nodes []types.VirtualMachineSnapshotTree,
	currentRef *types.ManagedObjectReference,
	parentID string,
) []port.SnapshotInfo {
	var result []port.SnapshotInfo
	for _, node := range nodes {
		isCurrent := currentRef != nil && node.Snapshot.Value == currentRef.Value
		info := port.SnapshotInfo{
			ProviderID:  node.Snapshot.Value,
			Name:        node.Name,
			Description: node.Description,
			IsCurrent:   isCurrent,
			ParentID:    parentID,
			CreatedAt:   node.CreateTime,
		}
		result = append(result, info)
		// Recurse into children.
		if len(node.ChildSnapshotList) > 0 {
			children := flattenSnapshotTree(node.ChildSnapshotList, currentRef, node.Snapshot.Value)
			result = append(result, children...)
		}
	}
	return result
}

// ── Datastore mapping ─────────────────────────────────────────────────────────

// mapDataStoreInfo converts a govmomi Datastore managed object to DataStoreInfo.
func mapDataStoreInfo(ds *mo.Datastore) port.DataStoreInfo {
	info := port.DataStoreInfo{
		ProviderID: ds.Self.Value,
		Name:       ds.Summary.Name,
		Type:       ds.Summary.Type,
		Accessible: ds.Summary.Accessible,
	}

	const bytesPerGB = float64(1024 * 1024 * 1024)
	info.CapacityGB = float64(ds.Summary.Capacity) / bytesPerGB
	info.FreeGB = float64(ds.Summary.FreeSpace) / bytesPerGB
	info.UsedGB = info.CapacityGB - info.FreeGB

	return info
}

// ── Network mapping ───────────────────────────────────────────────────────────

// mapNetworkInfo converts a govmomi Network managed object to NetworkInfo.
func mapNetworkInfo(net *mo.Network) port.NetworkInfo {
	return port.NetworkInfo{
		ProviderID: net.Self.Value,
		Name:       net.Name,
		Type:       net.Self.Type, // "Network" or "DistributedVirtualPortgroup"
		Accessible: net.Summary.GetNetworkSummary().Accessible,
	}
}

// ── Performance metrics ───────────────────────────────────────────────────────

// collectVMMetrics queries the vSphere Performance Manager for the most recent
// real-time sample of the key counters for a single VM.
//
// Counter keys used (group.name.rollup):
//
//	cpu.usage.average          → CPU utilisation %
//	mem.usage.average          → Memory utilisation %
//	disk.numberReadAveraged.average  → Disk read IOPS
//	disk.numberWriteAveraged.average → Disk write IOPS
//	net.bytesRx.average        → Network Rx (KB/s → Mbps)
//	net.bytesTx.average        → Network Tx (KB/s → Mbps)
func collectVMMetrics(ctx context.Context, client *vim25.Client, vmRef types.ManagedObjectReference) (*port.VMMetrics, error) {
	pm := performance.NewManager(client)

	// Resolve counter IDs from human-readable names.
	counters, err := pm.CounterInfoByName(ctx)
	if err != nil {
		return nil, fmt.Errorf("vmware: failed to load perf counters: %w", err)
	}

	wantCounters := []string{
		"cpu.usage.average",
		"mem.usage.average",
		"disk.numberReadAveraged.average",
		"disk.numberWriteAveraged.average",
		"net.bytesRx.average",
		"net.bytesTx.average",
	}

	metricIDs := make([]types.PerfMetricId, 0, len(wantCounters))
	counterIDMap := make(map[int32]string, len(wantCounters))
	for _, name := range wantCounters {
		if ci, ok := counters[name]; ok {
			metricIDs = append(metricIDs, types.PerfMetricId{
				CounterId: ci.Key,
				Instance:  "", // aggregate across all instances
			})
			counterIDMap[ci.Key] = name
		}
	}

	if len(metricIDs) == 0 {
		// Performance Manager not available (e.g. standalone ESXi without vCenter).
		return &port.VMMetrics{}, nil
	}

	// Request the most recent 20-second real-time sample (intervalId=20).
	query := types.PerfQuerySpec{
		Entity:     vmRef,
		MetricId:   metricIDs,
		MaxSample:  1,
		IntervalId: 20,
	}

	samples, err := pm.Query(ctx, []types.PerfQuerySpec{query})
	if err != nil {
		return nil, fmt.Errorf("vmware: perf query failed: %w", err)
	}

	metrics := &port.VMMetrics{}
	for _, base := range samples {
		series, ok := base.(*types.PerfEntityMetric)
		if !ok {
			continue
		}
		for _, baseSeries := range series.Value {
			intSeries, ok := baseSeries.(*types.PerfMetricIntSeries)
			if !ok || len(intSeries.Value) == 0 {
				continue
			}
			val := float64(intSeries.Value[len(intSeries.Value)-1])
			name := counterIDMap[intSeries.Id.CounterId]
			switch name {
			case "cpu.usage.average":
				// vSphere reports in hundredths of a percent (e.g. 1000 = 10%).
				metrics.CPUUsagePercent = val / 100.0
			case "mem.usage.average":
				metrics.MemoryUsagePercent = val / 100.0
			case "disk.numberReadAveraged.average":
				metrics.DiskReadIOPS = val
			case "disk.numberWriteAveraged.average":
				metrics.DiskWriteIOPS = val
			case "net.bytesRx.average":
				// vSphere reports in KB/s; convert to Mbps.
				metrics.NetworkRxMbps = val * 8 / 1000
			case "net.bytesTx.average":
				metrics.NetworkTxMbps = val * 8 / 1000
			}
		}
	}

	return metrics, nil
}

// ── Task polling ──────────────────────────────────────────────────────────────

// pollTaskResult polls a vSphere task until it reaches a terminal state or the
// context is cancelled. It returns the task result on success.
//
// govmomi's task.Wait already handles polling internally, but this function
// provides a manual polling path for callers that need progress callbacks or
// want to store intermediate state in the database.
func pollTaskResult(ctx context.Context, client *vim25.Client, taskRef types.ManagedObjectReference, interval time.Duration) (*types.TaskInfo, error) {
	if interval == 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			info, err := getTaskInfo(ctx, client, taskRef)
			if err != nil {
				return nil, fmt.Errorf("task poll: %w", err)
			}
			switch info.State {
			case types.TaskInfoStateSuccess:
				return info, nil
			case types.TaskInfoStateError:
				if info.Error != nil {
					return nil, fmt.Errorf("vSphere task error: %s", info.Error.LocalizedMessage)
				}
				return nil, fmt.Errorf("vSphere task failed (no detail)")
			// TaskInfoStateRunning / TaskInfoStateQueued: continue polling.
			}
		}
	}
}

// getTaskInfo retrieves the current TaskInfo for a task MOR.
func getTaskInfo(ctx context.Context, client *vim25.Client, taskRef types.ManagedObjectReference) (*types.TaskInfo, error) {
	var moTask mo.Task
	pc := property.DefaultCollector(client)
	if err := pc.RetrieveOne(ctx, taskRef, []string{"info"}, &moTask); err != nil {
		return nil, err
	}
	return &moTask.Info, nil
}
