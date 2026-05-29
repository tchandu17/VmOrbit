package nutanix

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

// mapVMToInfo converts a nutanixVM to the provider-agnostic VMInfo.
// The clusterMap maps cluster UUID → cluster name for enrichment.
// The hostMap maps host UUID → host name for enrichment.
func mapVMToInfo(vm *nutanixVM, clusterMap, hostMap map[string]string) port.VMInfo {
	resources := vm.Status.Resources

	// CPU: sockets × vCPUs per socket
	cpuCount := resources.NumSockets * resources.NumVCPUs
	if cpuCount == 0 {
		// Fall back to spec if status is empty
		cpuCount = vm.Spec.Resources.NumSockets * vm.Spec.Resources.NumVCPUs
	}

	// Memory
	memoryMB := resources.MemorySizeMiB
	if memoryMB == 0 {
		memoryMB = vm.Spec.Resources.MemorySizeMiB
	}

	// Disk: sum all DISK type disks
	diskGB := 0
	for _, d := range resources.DiskList {
		if d.DeviceProperties.DeviceType == "DISK" {
			diskGB += int(d.DiskSizeBytes / (1024 * 1024 * 1024))
			if diskGB == 0 && d.DiskSizeMiB > 0 {
				diskGB += d.DiskSizeMiB / 1024
			}
		}
	}

	// IP addresses and MAC from NICs
	var ipAddresses []string
	var macAddress string
	var networkName string
	for i, nic := range resources.NicList {
		if i == 0 {
			macAddress = nic.MACAddress
			if nic.SubnetReference != nil {
				networkName = nic.SubnetReference.Name
			}
		}
		for _, ep := range nic.IPEndpointList {
			if ep.IP != "" {
				ipAddresses = append(ipAddresses, ep.IP)
			}
		}
	}

	// Cluster and host names
	clusterName := ""
	if vm.Status.ClusterReference != nil {
		clusterName = vm.Status.ClusterReference.Name
		if clusterName == "" {
			clusterName = clusterMap[vm.Status.ClusterReference.UUID]
		}
	}

	hostName := ""
	if resources.HostReference != nil {
		hostName = resources.HostReference.Name
		if hostName == "" {
			hostName = hostMap[resources.HostReference.UUID]
		}
	}

	// Guest OS
	guestOS := ""
	if resources.GuestOS != nil {
		guestOS = mapGuestOS(resources.GuestOS.ID)
	}

	extra := map[string]interface{}{
		"cluster":      clusterName,
		"host":         hostName,
		"hypervisor":   resources.HypervisorType,
		"power_state":  resources.PowerState,
	}
	if vm.Status.ClusterReference != nil {
		extra["cluster_uuid"] = vm.Status.ClusterReference.UUID
	}
	if resources.HostReference != nil {
		extra["host_uuid"] = resources.HostReference.UUID
	}

	return port.VMInfo{
		ProviderVMID: vm.Metadata.UUID,
		Name:         vm.Status.Name,
		Status:       mapPowerState(resources.PowerState),
		CPUCount:     cpuCount,
		MemoryMB:     memoryMB,
		DiskGB:       diskGB,
		IPAddresses:  ipAddresses,
		MACAddress:   macAddress,
		NetworkName:  networkName,
		GuestOS:      guestOS,
		GuestOSType:  guestOS,
		Extra:        extra,
	}
}

// mapPowerState converts a Nutanix power state string to the domain VMStatus.
func mapPowerState(state string) model.VMStatus {
	switch strings.ToUpper(state) {
	case "ON":
		return model.VMStatusRunning
	case "OFF":
		return model.VMStatusStopped
	case "PAUSED", "SUSPEND":
		return model.VMStatusSuspended
	case "UNKNOWN":
		return model.VMStatusUnknown
	default:
		if state == "" {
			return model.VMStatusUnknown
		}
		return model.VMStatusUnknown
	}
}

// mapGuestOS converts a Nutanix guest OS ID to a human-readable name.
func mapGuestOS(osID string) string {
	if osID == "" {
		return "Unknown"
	}
	lower := strings.ToLower(osID)
	switch {
	case strings.Contains(lower, "windows"):
		return "Windows"
	case strings.Contains(lower, "linux"):
		return "Linux"
	case strings.Contains(lower, "centos"):
		return "CentOS"
	case strings.Contains(lower, "ubuntu"):
		return "Ubuntu"
	case strings.Contains(lower, "rhel") || strings.Contains(lower, "redhat"):
		return "Red Hat Enterprise Linux"
	case strings.Contains(lower, "debian"):
		return "Debian"
	case strings.Contains(lower, "suse"):
		return "SUSE Linux"
	default:
		return osID
	}
}

// vmStatusToPowerTransition maps a desired VMStatus to the Nutanix power transition string.
func vmStatusToPowerTransition(action string) string {
	switch strings.ToLower(action) {
	case "power_on", "on":
		return "ON"
	case "power_off", "off":
		return "OFF"
	case "reboot", "restart":
		return "ACPI_REBOOT"
	case "reset":
		return "POWERCYCLE"
	case "suspend", "pause":
		return "PAUSE"
	case "resume":
		return "RESUME"
	case "shutdown":
		return "ACPI_SHUTDOWN"
	default:
		return strings.ToUpper(action)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Snapshot mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapSnapshot converts a nutanixVMSnapshot to the provider-agnostic SnapshotInfo.
func mapSnapshot(s *nutanixVMSnapshot) port.SnapshotInfo {
	var createdAt time.Time
	if s.CreationTime > 0 {
		// Nutanix returns creation_time in microseconds since epoch
		createdAt = time.Unix(s.CreationTime/1_000_000, 0).UTC()
	}
	return port.SnapshotInfo{
		ProviderID:  s.UUID,
		Name:        s.Name,
		Description: s.ConsistencyGroup,
		IsCurrent:   false, // Nutanix doesn't have a "current" concept
		CreatedAt:   createdAt,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Storage mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapStorageContainer converts a nutanixStorageContainer to the provider-agnostic DataStoreInfo.
func mapStorageContainer(s *nutanixStorageContainer) port.DataStoreInfo {
	const bytesPerGB = float64(1024 * 1024 * 1024)

	capacityGB := float64(s.MaxCapacity) / bytesPerGB

	// Parse usage stats (stored as string bytes in Nutanix API)
	usedBytes := parseStatBytes(s.UsageStats.StorageUsageBytes)
	freeBytes := parseStatBytes(s.UsageStats.StorageFreeBytes)

	usedGB := float64(usedBytes) / bytesPerGB
	freeGB := float64(freeBytes) / bytesPerGB

	// If free is not reported, calculate from capacity - used
	if freeGB == 0 && capacityGB > 0 {
		freeGB = capacityGB - usedGB
	}

	return port.DataStoreInfo{
		ProviderID: s.ContainerUUID,
		Name:       s.Name,
		Type:       "nutanix_container",
		CapacityGB: capacityGB,
		UsedGB:     usedGB,
		FreeGB:     freeGB,
		Accessible: true,
	}
}

// parseStatBytes parses a Nutanix usage stat string (may be empty or a number).
func parseStatBytes(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ─────────────────────────────────────────────────────────────────────────────
// Network mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapSubnet converts a nutanixSubnet to the provider-agnostic NetworkInfo.
func mapSubnet(s *nutanixSubnet) port.NetworkInfo {
	return port.NetworkInfo{
		ProviderID: s.Metadata.UUID,
		Name:       s.Status.Name,
		Type:       strings.ToLower(s.Status.Resources.SubnetType),
		VLAN:       s.Status.Resources.VlanID,
		Accessible: s.Status.State == "COMPLETE" || s.Status.State == "ACTIVE",
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Template (Image) mapping
// ─────────────────────────────────────────────────────────────────────────────

// mapImageToTemplate converts a nutanixImage to the provider-agnostic TemplateInfo.
func mapImageToTemplate(img *nutanixImage) port.TemplateInfo {
	diskGB := int(img.Status.Resources.SizeBytes / (1024 * 1024 * 1024))
	return port.TemplateInfo{
		ProviderID:  img.Metadata.UUID,
		Name:        img.Status.Name,
		Description: img.Status.Description,
		GuestOS:     img.Status.Resources.Architecture,
		DiskGB:      diskGB,
		Extra: map[string]interface{}{
			"image_type":   img.Status.Resources.ImageType,
			"architecture": img.Status.Resources.Architecture,
			"size_bytes":   img.Status.Resources.SizeBytes,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// VM create spec builder
// ─────────────────────────────────────────────────────────────────────────────

// buildVMCreateSpec builds the Nutanix v3 VM create request body from a VMCreateSpec.
func buildVMCreateSpec(spec port.VMCreateSpec, clusterUUID string) map[string]interface{} {
	resources := map[string]interface{}{
		"num_vcpus_per_socket": spec.CPUCount,
		"num_sockets":          1,
		"memory_size_mib":      spec.MemoryMB,
		"power_state":          "OFF",
		"hypervisor_type":      "AHV",
	}

	// Disk
	if spec.DiskGB > 0 {
		diskList := []map[string]interface{}{
			{
				"device_properties": map[string]interface{}{
					"device_type": "DISK",
					"disk_address": map[string]interface{}{
						"adapter_type": "SCSI",
						"device_index": 0,
					},
				},
				"disk_size_mib": spec.DiskGB * 1024,
			},
		}
		// If a template image is specified, use it as the data source
		if spec.TemplateID != "" {
			diskList[0]["data_source_reference"] = map[string]interface{}{
				"kind": "image",
				"uuid": spec.TemplateID,
			}
		}
		resources["disk_list"] = diskList
	}

	// NIC
	if spec.NetworkName != "" {
		resources["nic_list"] = []map[string]interface{}{
			{
				"nic_type": "NORMAL_NIC",
				"subnet_reference": map[string]interface{}{
					"kind": "subnet",
					"name": spec.NetworkName,
				},
			},
		}
	}

	body := map[string]interface{}{
		"spec": map[string]interface{}{
			"name":        spec.Name,
			"description": "",
			"resources":   resources,
		},
		"metadata": map[string]interface{}{
			"kind": "vm",
		},
	}

	// Attach to cluster if specified
	if clusterUUID != "" {
		body["spec"].(map[string]interface{})["cluster_reference"] = map[string]interface{}{
			"kind": "cluster",
			"uuid": clusterUUID,
		}
	}

	return body
}

// buildVMCloneSpec builds the Nutanix v3 VM clone request body.
func buildVMCloneSpec(sourceVM *nutanixVM, spec port.VMCloneSpec) map[string]interface{} {
	// Deep copy the source VM spec and override the name
	resources := map[string]interface{}{
		"num_vcpus_per_socket": sourceVM.Spec.Resources.NumVCPUs,
		"num_sockets":          sourceVM.Spec.Resources.NumSockets,
		"memory_size_mib":      sourceVM.Spec.Resources.MemorySizeMiB,
		"power_state":          "OFF",
		"hypervisor_type":      "AHV",
	}

	// Clone disks from source
	if len(sourceVM.Spec.Resources.DiskList) > 0 {
		diskList := make([]map[string]interface{}, 0, len(sourceVM.Spec.Resources.DiskList))
		for _, d := range sourceVM.Spec.Resources.DiskList {
			disk := map[string]interface{}{
				"device_properties": map[string]interface{}{
					"device_type": d.DeviceProperties.DeviceType,
					"disk_address": map[string]interface{}{
						"adapter_type": d.DeviceProperties.DiskAddress.AdapterType,
						"device_index": d.DeviceProperties.DiskAddress.DeviceIndex,
					},
				},
				"disk_size_mib": d.DiskSizeMiB,
			}
			if d.DataSourceReference != nil {
				disk["data_source_reference"] = map[string]interface{}{
					"kind": d.DataSourceReference.Kind,
					"uuid": d.DataSourceReference.UUID,
				}
			}
			diskList = append(diskList, disk)
		}
		resources["disk_list"] = diskList
	}

	// Clone NICs from source
	if len(sourceVM.Spec.Resources.NicList) > 0 {
		nicList := make([]map[string]interface{}, 0, len(sourceVM.Spec.Resources.NicList))
		for _, nic := range sourceVM.Spec.Resources.NicList {
			n := map[string]interface{}{
				"nic_type": nic.NICType,
			}
			if nic.SubnetReference != nil {
				n["subnet_reference"] = map[string]interface{}{
					"kind": nic.SubnetReference.Kind,
					"uuid": nic.SubnetReference.UUID,
				}
			}
			nicList = append(nicList, n)
		}
		resources["nic_list"] = nicList
	}

	body := map[string]interface{}{
		"spec": map[string]interface{}{
			"name":        spec.Name,
			"description": fmt.Sprintf("Clone of %s", sourceVM.Status.Name),
			"resources":   resources,
		},
		"metadata": map[string]interface{}{
			"kind": "vm",
		},
	}

	// Preserve cluster reference
	if sourceVM.Spec.ClusterReference != nil {
		body["spec"].(map[string]interface{})["cluster_reference"] = map[string]interface{}{
			"kind": sourceVM.Spec.ClusterReference.Kind,
			"uuid": sourceVM.Spec.ClusterReference.UUID,
		}
	}

	return body
}
