package model

// ─────────────────────────────────────────────────────────────────────────────
// Infrastructure hierarchy models
//
// These models store the physical/logical topology discovered during inventory
// sync. They complement the existing Hypervisor, DataStore, and Network models
// by adding Cluster and Host layers.
//
// Hierarchy:
//   Hypervisor (Provider)
//     └── Cluster (ClusterComputeResource / Proxmox cluster)
//           └── Host (HostSystem / Proxmox node)
//                 ├── VMs (via vms.metadata esxi_host / node)
//                 ├── DataStores (via datastores.hypervisor_id)
//                 └── Networks (via networks.hypervisor_id)
// ─────────────────────────────────────────────────────────────────────────────

// Cluster represents a compute cluster discovered from a hypervisor.
// For VMware this is a ClusterComputeResource; for Proxmox it is the cluster
// name reported by /cluster/status.
type Cluster struct {
	Base
	HypervisorID string  `gorm:"type:uuid;not null;index;uniqueIndex:uidx_cluster_provider" json:"hypervisor_id"`
	ProviderID   string  `gorm:"not null;size:256;uniqueIndex:uidx_cluster_provider"        json:"provider_id"` // MOR value or cluster name
	Name         string  `gorm:"not null;size:256;index"                                    json:"name"`
	TotalCPU     int     `gorm:"not null;default:0"                                         json:"total_cpu"`
	TotalMemoryMB int    `gorm:"not null;default:0"                                         json:"total_memory_mb"`
	HostCount    int     `gorm:"not null;default:0"                                         json:"host_count"`
	VMCount      int     `gorm:"not null;default:0"                                         json:"vm_count"`
	Metadata     JSONMap `gorm:"type:jsonb"                                                 json:"metadata"`

	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
	Hosts      []Host     `gorm:"foreignKey:ClusterID"    json:"hosts,omitempty"`
}

// Host represents a physical hypervisor host (ESXi host or Proxmox node).
// For VMware this is a HostSystem; for Proxmox it is a node from /nodes.
type Host struct {
	Base
	HypervisorID string  `gorm:"type:uuid;not null;index;uniqueIndex:uidx_host_provider" json:"hypervisor_id"`
	ClusterID    *string `gorm:"type:uuid;index"                                         json:"cluster_id,omitempty"`
	ProviderID   string  `gorm:"not null;size:256;uniqueIndex:uidx_host_provider"        json:"provider_id"` // MOR value or node name
	Name         string  `gorm:"not null;size:256;index"                                 json:"name"`
	Status       string  `gorm:"not null;default:'unknown';size:32;index"                json:"status"` // connected/disconnected/maintenance/unknown
	// Compute
	CPUModel     string  `gorm:"size:256"           json:"cpu_model"`
	CPUSockets   int     `gorm:"not null;default:0" json:"cpu_sockets"`
	CPUCores     int     `gorm:"not null;default:0" json:"cpu_cores"`
	CPUThreads   int     `gorm:"not null;default:0" json:"cpu_threads"`
	CPUUsageMHz  int     `gorm:"not null;default:0" json:"cpu_usage_mhz"`
	TotalMemoryMB int    `gorm:"not null;default:0" json:"total_memory_mb"`
	UsedMemoryMB  int    `gorm:"not null;default:0" json:"used_memory_mb"`
	// Info
	HypervisorVersion string  `gorm:"size:128"           json:"hypervisor_version"`
	UptimeSeconds     int64   `gorm:"not null;default:0" json:"uptime_seconds"`
	VMCount           int     `gorm:"not null;default:0" json:"vm_count"`
	Metadata          JSONMap `gorm:"type:jsonb"         json:"metadata"`

	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
	Cluster    *Cluster   `gorm:"foreignKey:ClusterID"    json:"cluster,omitempty"`
}
