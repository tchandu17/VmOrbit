package model

import "github.com/google/uuid"

// ─────────────────────────────────────────────────────────────────────────────
// Enumerations
// ─────────────────────────────────────────────────────────────────────────────

// VMStatus represents the power state of a virtual machine.
type VMStatus string

const (
	VMStatusRunning      VMStatus = "running"
	VMStatusStopped      VMStatus = "stopped"
	VMStatusSuspended    VMStatus = "suspended"
	VMStatusPaused       VMStatus = "paused"
	VMStatusUnknown      VMStatus = "unknown"
	VMStatusProvisioning VMStatus = "provisioning"
	VMStatusDeleting     VMStatus = "deleting"
	VMStatusError        VMStatus = "error"
)

// ─────────────────────────────────────────────────────────────────────────────
// VMTemplate — reusable provisioning blueprint
// ─────────────────────────────────────────────────────────────────────────────

// VMTemplate is a provider-side template or golden image that can be cloned
// into new VMs. Templates are discovered during inventory sync.
// The composite unique index on (hypervisor_id, provider_id) prevents
// duplicate template records for the same provider-native object.
type VMTemplate struct {
	Base
	HypervisorID uuid.UUID   `gorm:"type:uuid;not null;index;uniqueIndex:uidx_template_provider" json:"hypervisor_id"`
	ProviderID   string      `gorm:"not null;size:256;uniqueIndex:uidx_template_provider"        json:"provider_id"`
	Name         string      `gorm:"not null;size:256;index"                                     json:"name"`
	Description  string      `gorm:"size:512"                                                    json:"description"`
	GuestOS      string      `gorm:"size:128"                                                    json:"guest_os"`
	CPUCount     int         `gorm:"not null;default:0"                                          json:"cpu_count"`
	MemoryMB     int         `gorm:"not null;default:0"                                          json:"memory_mb"`
	DiskGB       int         `gorm:"not null;default:0"                                          json:"disk_gb"`
	Tags         StringArray `gorm:"type:text[]"                                                 json:"tags"`
	Metadata     JSONMap     `gorm:"type:jsonb"                                                  json:"metadata"`

	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// VM
// ─────────────────────────────────────────────────────────────────────────────

// VM represents a virtual machine in the inventory.
// The composite unique index on (hypervisor_id, provider_vm_id) ensures that
// the same provider-native VM is never duplicated in the database.
type VM struct {
	Base
	HypervisorID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uidx_vm_provider" json:"hypervisor_id"`
	ProviderVMID string    `gorm:"not null;size:256;uniqueIndex:uidx_vm_provider"        json:"provider_vm_id"` // native ID from provider
	Name         string    `gorm:"not null;size:256;index"                               json:"name"`
	Description  string    `gorm:"size:512"                                              json:"description"`
	Status       VMStatus  `gorm:"not null;default:'unknown';index;size:32"              json:"status"`

	// Compute
	CPUCount int `gorm:"not null;default:0" json:"cpu_count"`
	MemoryMB int `gorm:"not null;default:0" json:"memory_mb"`
	DiskGB   int `gorm:"not null;default:0" json:"disk_gb"`

	// Network
	IPAddresses StringArray `gorm:"type:text[]" json:"ip_addresses"`
	MACAddress  string      `gorm:"size:17"     json:"mac_address"`
	NetworkName string      `gorm:"size:256"    json:"network_name"`

	// OS
	GuestOS     string `gorm:"size:128" json:"guest_os"`
	GuestOSType string `gorm:"size:64"  json:"guest_os_type"`
	ToolsStatus string `gorm:"size:64"  json:"tools_status"`

  // Metadata
	Tags     StringArray `gorm:"type:text[]" json:"tags"`
	Metadata JSONMap     `gorm:"type:jsonb"  json:"metadata"`

	// Relations
	Hypervisor Hypervisor `gorm:"foreignKey:HypervisorID" json:"hypervisor,omitempty"`
	Snapshots  []Snapshot `gorm:"foreignKey:VMID"         json:"snapshots,omitempty"`
	TagObjects []Tag      `gorm:"many2many:vm_tags;"      json:"tag_objects,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Snapshot
// ─────────────────────────────────────────────────────────────────────────────

// Snapshot represents a VM snapshot.
// ParentID is nullable — a nil parent means this is a root snapshot.
// The self-referential ParentID enables tree traversal of snapshot chains.
type Snapshot struct {
	Base
	VMID       uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:uidx_snapshot_provider" json:"vm_id"`
	ProviderID string     `gorm:"not null;size:256;uniqueIndex:uidx_snapshot_provider"        json:"provider_id"`
	Name       string     `gorm:"not null;size:256"                                           json:"name"`
	Description string    `gorm:"size:512"                                                    json:"description"`
	IsCurrent  bool       `gorm:"not null;default:false;index"                                json:"is_current"`
	ParentID   *uuid.UUID `gorm:"type:uuid;index"                                             json:"parent_id,omitempty"` // self-referential

	VM     VM        `gorm:"foreignKey:VMID"     json:"vm,omitempty"`
	Parent *Snapshot `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
}
