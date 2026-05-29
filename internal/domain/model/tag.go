package model

import "github.com/google/uuid"

// ─────────────────────────────────────────────────────────────────────────────
// Tag — a named, color-coded label that can be attached to VMs
// ─────────────────────────────────────────────────────────────────────────────

// Tag is a reusable label with an optional color.
// Tags are stored in their own table and associated with VMs via the vm_tags
// join table. This allows rich metadata (color, description) and efficient
// filtering by tag across the entire inventory.
type Tag struct {
	Base
	Name        string `gorm:"not null;size:64;uniqueIndex" json:"name"`
	Color       string `gorm:"not null;size:16;default:'#6B7280'" json:"color"` // hex color
	Description string `gorm:"size:256" json:"description"`

	// Many-to-many: a tag can be on many VMs, a VM can have many tags.
	VMs []VM `gorm:"many2many:vm_tags;" json:"vms,omitempty"`
}

// VMTag is the explicit join table for the VM ↔ Tag many-to-many relation.
// GORM will manage this automatically via the many2many tag, but declaring it
// explicitly lets us add extra columns (e.g. tagged_by) in the future.
type VMTag struct {
	VMID  uuid.UUID `gorm:"type:uuid;primaryKey" json:"vm_id"`
	TagID uuid.UUID `gorm:"type:uuid;primaryKey" json:"tag_id"`
}
