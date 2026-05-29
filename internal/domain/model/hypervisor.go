package model

import "time"

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// ProviderType identifies the hypervisor vendor.
type ProviderType string

const (
	ProviderVMware  ProviderType = "vmware"  // VMware vCenter
	ProviderESXi    ProviderType = "esxi"    // VMware ESXi (standalone host, no vCenter)
	ProviderProxmox ProviderType = "proxmox"
	ProviderNutanix ProviderType = "nutanix" // Nutanix AHV (Prism Element / Prism Central)
	ProviderKVM     ProviderType = "kvm"
	ProviderHyperV  ProviderType = "hyperv"
)

// ConnectionStatus reflects the current reachability of a hypervisor.
type ConnectionStatus string

const (
	ConnectionStatusConnected    ConnectionStatus = "connected"
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
	ConnectionStatusError        ConnectionStatus = "error"
	ConnectionStatusUnknown      ConnectionStatus = "unknown"
)

// ─────────────────────────────────────────────────────────────────────────────
// HypervisorGroup — optional logical grouping (datacenter / cluster / region)
// ─────────────────────────────────────────────────────────────────────────────

// HypervisorGroup is an optional organisational container for hypervisors.
// Examples: "DC-East", "Prod-Cluster-1", "EU-West".
type HypervisorGroup struct {
	Base
	Name        string       `gorm:"uniqueIndex;not null;size:128" json:"name"`
	Description string       `gorm:"size:512"                      json:"description"`
	Tags        StringArray  `gorm:"type:text[]"                   json:"tags"`
	Hypervisors []Hypervisor `gorm:"foreignKey:GroupID"            json:"hypervisors,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Hypervisor
// ─────────────────────────────────────────────────────────────────────────────

// Hypervisor represents a registered hypervisor host.
// The composite unique index on (host, port, provider) prevents duplicate
// registrations of the same endpoint.
type Hypervisor struct {
	Base
	GroupID          *string          `gorm:"type:uuid;index"              json:"group_id,omitempty"`
	Name             string           `gorm:"not null;size:128"            json:"name"`
	Description      string           `gorm:"size:512"                     json:"description"`
	Provider         ProviderType     `gorm:"not null;index;size:32"       json:"provider"`
	Host             string           `gorm:"not null;size:253"            json:"host"`
	Port             int              `gorm:"not null"                     json:"port"`
	Username         string           `gorm:"size:128"                     json:"username"`
	EncryptedSecret  string           `gorm:"not null"                     json:"-"` // AES-GCM encrypted
	TLSVerify        bool             `gorm:"not null;default:false"       json:"tls_verify"`
	ConnectionStatus ConnectionStatus `gorm:"not null;default:'unknown';index;size:32" json:"connection_status"`
	LastCheckedAt    *time.Time       `gorm:"index"                        json:"last_checked_at,omitempty"`
	Tags             StringArray      `gorm:"type:text[]"                  json:"tags"`
	Metadata         JSONMap          `gorm:"type:jsonb"                   json:"metadata"`

	// Relations
	Group      *HypervisorGroup `gorm:"foreignKey:GroupID"      json:"group,omitempty"`
	VMs        []VM             `gorm:"foreignKey:HypervisorID" json:"vms,omitempty"`
	DataStores []DataStore      `gorm:"foreignKey:HypervisorID" json:"datastores,omitempty"`
	Networks   []Network        `gorm:"foreignKey:HypervisorID" json:"networks,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Storage & Network resources
// ─────────────────────────────────────────────────────────────────────────────

// DataStore represents a storage resource on a hypervisor.
type DataStore struct {
	Base
	HypervisorID string  `gorm:"type:uuid;not null;index;uniqueIndex:uidx_ds_provider"  json:"hypervisor_id"`
	ProviderID   string  `gorm:"not null;size:256;uniqueIndex:uidx_ds_provider"         json:"provider_id"` // native ID from provider
	Name         string  `gorm:"not null;size:256"                                      json:"name"`
	Type         string  `gorm:"size:64"                                                json:"type"` // nfs, local, iscsi, ceph
	CapacityGB   float64 `gorm:"not null;default:0"                                     json:"capacity_gb"`
	UsedGB       float64 `gorm:"not null;default:0"                                     json:"used_gb"`
	FreeGB       float64 `gorm:"not null;default:0"                                     json:"free_gb"`
	Accessible   bool    `gorm:"not null;default:true;index"                            json:"accessible"`

	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
}

// Network represents a virtual network on a hypervisor.
type Network struct {
	Base
	HypervisorID string `gorm:"type:uuid;not null;index;uniqueIndex:uidx_net_provider"  json:"hypervisor_id"`
	ProviderID   string `gorm:"not null;size:256;uniqueIndex:uidx_net_provider"         json:"provider_id"` // native ID from provider
	Name         string `gorm:"not null;size:256"                                       json:"name"`
	Type         string `gorm:"size:64"                                                 json:"type"` // standard, distributed, bridge
	VLAN         int    `gorm:"default:0"                                              json:"vlan"`
	Accessible   bool   `gorm:"not null;default:true;index"                            json:"accessible"`

	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
}
