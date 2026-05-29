package proxmox

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ─────────────────────────────────────────────────────────────────────────────
// VM mapping
// ─────────────────────────────────────────────────────────────────────────────

// buildProviderVMID builds the canonical provider VM ID used throughout the system.
// Format: "<node>/<vmid>" — this lets us recover both pieces without extra
// lookups when routing API calls.
func buildProviderVMID(node string, vmid int) string {
	return fmt.Sprintf("%s/%d", node, vmid)
}

// parseProviderVMID splits a "<node>/<vmid>" string back into its parts.
func parseProviderVMID(id string) (node string, vmid int, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("proxmox: invalid provider VM ID %q (expected node/vmid)", id)
	}
	vmid, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("proxmox: invalid vmid in %q: %w", id, err)
	}
	return parts[0], vmid, nil
}

// mapResourceToVMInfo converts a pveResource (from cluster/resources) to a
// provider-agnostic VMInfo. Only "qemu" type resources are meaningful here.
func mapResourceToVMInfo(r *pveResource) port.VMInfo {
	return port.VMInfo{
		ProviderVMID: buildProviderVMID(r.Node, r.VMID),
		Name:         r.Name,
		Status:       mapVMStatus(r.Status),
		CPUCount:     r.MaxCPU,
		MemoryMB:     int(r.MaxMem / (1024 * 1024)),
		DiskGB:       int(r.MaxDisk / (1024 * 1024 * 1024)),
		Extra: map[string]interface{}{
			"node":       r.Node,
			"vmid":       r.VMID,
			"uptime":     r.Uptime,
			"cpu_usage":  r.CPU,
			"mem_used":   r.Mem,
			"disk_read":  r.DiskRead,
			"disk_write": r.DiskWrite,
			"net_in":     r.NetIn,
			"net_out":    r.NetOut,
		},
	}
}

// enrichVMInfo merges detailed config and status data into an existing VMInfo.
// Call this after mapResourceToVMInfo when you need full VM details.
func enrichVMInfo(info *port.VMInfo, status *pveVMStatus, cfg *pveVMConfig) {
	if status != nil {
		info.Status = mapVMStatus(status.Status)
		info.CPUCount = status.CPUs
		info.MemoryMB = int(status.MaxMem / (1024 * 1024))
		info.DiskGB = int(status.MaxDisk / (1024 * 1024 * 1024))
		if info.Name == "" {
			info.Name = status.Name
		}
	}

	if cfg != nil {
		if info.Name == "" {
			info.Name = cfg.Name
		}
		info.GuestOS = mapOSType(cfg.OSType)
		info.GuestOSType = cfg.OSType

		// Parse CPU: cores × sockets.
		if cfg.Cores > 0 && cfg.Sockets > 0 {
			info.CPUCount = cfg.Cores * cfg.Sockets
		} else if cfg.Cores > 0 {
			info.CPUCount = cfg.Cores
		}

		// Memory from config is already in MB.
		if cfg.Memory > 0 {
			info.MemoryMB = cfg.Memory
		}

		// Extract MAC address and network name from the first net interface.
		mac, netName := parseNetInterface(cfg.Net0)
		if mac != "" {
			info.MACAddress = mac
		}
		if netName != "" {
			info.NetworkName = netName
		}

		// Extract disk size from the first disk.
		diskGB := parseDiskSize(cfg.Scsi0)
		if diskGB == 0 {
			diskGB = parseDiskSize(cfg.Virtio0)
		}
		if diskGB > 0 {
			info.DiskGB = diskGB
		}

		if info.Extra == nil {
			info.Extra = make(map[string]interface{})
		}
		info.Extra["description"] = cfg.Description
		info.Extra["agent"] = cfg.Agent
	}
}

// mapVMStatus converts a Proxmox VM status string to the domain VMStatus.
func mapVMStatus(status string) model.VMStatus {
	switch strings.ToLower(status) {
	case "running":
		return model.VMStatusRunning
	case "stopped":
		return model.VMStatusStopped
	case "paused":
		return model.VMStatusPaused
	case "suspended":
		return model.VMStatusSuspended
	default:
		return model.VMStatusUnknown
	}
}

// mapOSType converts a Proxmox ostype string to a human-readable OS name.
func mapOSType(ostype string) string {
	switch ostype {
	case "l24":
		return "Linux 2.4"
	case "l26":
		return "Linux 2.6+"
	case "win7", "win8", "win10", "win11":
		return "Windows " + strings.TrimPrefix(ostype, "win")
	case "w2k", "w2k3", "w2k8":
		return "Windows Server"
	case "solaris":
		return "Solaris"
	case "other":
		return "Other"
	default:
		if ostype != "" {
			return ostype
		}
		return "Unknown"
	}
}

// parseNetInterface extracts the MAC address and bridge/network name from a
// Proxmox net interface string, e.g.:
//
//	"virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1"
func parseNetInterface(netStr string) (mac, network string) {
	if netStr == "" {
		return "", ""
	}
	for _, part := range strings.Split(netStr, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			// The first token may be "<model>=<mac>" without a key prefix.
			// e.g. "virtio=AA:BB:CC:DD:EE:FF"
			if strings.Contains(part, ":") {
				// Looks like a MAC address embedded in the model field.
				inner := strings.SplitN(part, "=", 2)
				if len(inner) == 2 {
					mac = inner[1]
				}
			}
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "virtio", "e1000", "rtl8139", "vmxnet3":
			mac = kv[1]
		case "bridge":
			network = kv[1]
		}
	}
	return mac, network
}

// parseDiskSize extracts the disk size in GB from a Proxmox disk string, e.g.:
//
//	"local-lvm:vm-100-disk-0,size=32G"
//	"ceph:vm-100-disk-0,size=100G"
func parseDiskSize(diskStr string) int {
	if diskStr == "" {
		return 0
	}
	for _, part := range strings.Split(diskStr, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && strings.ToLower(kv[0]) == "size" {
			return parseSize(kv[1])
		}
	}
	return 0
}

// parseSize converts a Proxmox size string ("32G", "512M", "1T") to GB.
func parseSize(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	unit := strings.ToUpper(string(s[len(s)-1]))
	numStr := s[:len(s)-1]
	n, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "T":
		return int(n * 1024)
	case "G":
		return int(n)
	case "M":
		return int(n / 1024)
	case "K":
		return int(n / (1024 * 1024))
	default:
		// Assume bytes.
		return int(n / (1024 * 1024 * 1024))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Snapshot mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapSnapshot converts a pveSnapshot to the provider-agnostic SnapshotInfo.
// currentName is the name of the currently active snapshot (may be empty).
func mapSnapshot(s *pveSnapshot, currentName string) port.SnapshotInfo {
	var createdAt time.Time
	if s.SnapTime > 0 {
		createdAt = time.Unix(int64(s.SnapTime), 0).UTC()
	}
	return port.SnapshotInfo{
		ProviderID:  s.Name, // Proxmox uses the snapshot name as its ID
		Name:        s.Name,
		Description: s.Description,
		IsCurrent:   s.Name == currentName,
		ParentID:    s.Parent,
		CreatedAt:   createdAt,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Storage mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapStorage converts a pveStorage to the provider-agnostic DataStoreInfo.
func mapStorage(s *pveStorage, node string) port.DataStoreInfo {
	const bytesPerGB = float64(1024 * 1024 * 1024)
	capacityGB := float64(s.Total) / bytesPerGB
	usedGB := float64(s.Used) / bytesPerGB
	freeGB := float64(s.Avail) / bytesPerGB

	return port.DataStoreInfo{
		ProviderID: fmt.Sprintf("%s/%s", node, s.Storage),
		Name:       s.Storage,
		Type:       s.Type,
		CapacityGB: capacityGB,
		UsedGB:     usedGB,
		FreeGB:     freeGB,
		Accessible: s.Active == 1 && s.Enabled == 1,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Network mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapNetwork converts a pveNetwork to the provider-agnostic NetworkInfo.
// Only bridge-type interfaces are exposed as usable VM networks.
func mapNetwork(n *pveNetwork, node string) port.NetworkInfo {
	return port.NetworkInfo{
		ProviderID: fmt.Sprintf("%s/%s", node, n.Iface),
		Name:       n.Iface,
		Type:       n.Type,
		Accessible: n.Active == 1,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Metrics mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapRRDMetrics converts the most recent pveRRDData point to VMMetrics.
// RRD values are already normalised (CPU as fraction 0–1, memory in bytes).
func mapRRDMetrics(data []pveRRDData, maxMem int64) *port.VMMetrics {
	if len(data) == 0 {
		return &port.VMMetrics{}
	}
	// Use the most recent data point.
	d := data[len(data)-1]

	var memPct float64
	if maxMem > 0 && d.MaxMem > 0 {
		memPct = (d.Mem / d.MaxMem) * 100.0
	}

	const bytesPerMbps = float64(1024 * 1024 / 8)
	return &port.VMMetrics{
		CPUUsagePercent:    d.CPU * 100.0,
		MemoryUsagePercent: memPct,
		// Proxmox RRD reports disk I/O in bytes/s — convert to approximate IOPS
		// using a 4 KB block size assumption.
		DiskReadIOPS:  d.DiskRead / 4096,
		DiskWriteIOPS: d.DiskWrite / 4096,
		// Network is in bytes/s — convert to Mbps.
		NetworkRxMbps: d.NetIn / bytesPerMbps,
		NetworkTxMbps: d.NetOut / bytesPerMbps,
	}
}
