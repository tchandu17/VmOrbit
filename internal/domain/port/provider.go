package port

import (
	"context"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Provider — top-level hypervisor adapter contract
// ─────────────────────────────────────────────────────────────────────────────

// Provider is the single interface every hypervisor adapter must implement.
// New providers only need to satisfy this interface to be registered.
type Provider interface {
	// Meta
	Type() model.ProviderType
	Name() string

	// Capabilities advertises which optional feature sets this provider supports.
	// Callers must check before invoking optional sub-interfaces.
	Capabilities() ProviderCapabilities

	// Lifecycle
	Connect(ctx context.Context, creds Credentials) error
	Disconnect(ctx context.Context) error
	Ping(ctx context.Context) error

	// Core operations — every provider must implement these.
	VMProvider
	SnapshotProvider
	StorageProvider
	NetworkProvider
	InventoryProvider
}

// ConsoleProvider is an optional capability for providers that support
// interactive console sessions. Check Capabilities().Console before casting.
type ConsoleProvider interface {
	GetConsoleSession(ctx context.Context, providerVMID string, opts ConsoleOptions) (*ConsoleSession, error)
}

// Credentials carries the connection details for a hypervisor.
type Credentials struct {
	Host      string
	Port      int
	Username  string
	Password  string
	Token     string // API token alternative
	TLSVerify bool
	Extra     map[string]string // provider-specific extras
}

// ─────────────────────────────────────────────────────────────────────────────
// Capability model
// ─────────────────────────────────────────────────────────────────────────────

// ProviderCapabilities declares which optional features a provider supports.
// Handlers and services must check the relevant flag before calling optional
// sub-interfaces to avoid ErrUnsupported at runtime.
type ProviderCapabilities struct {
	// Console indicates the provider implements ConsoleProvider.
	Console bool
	// LinkedClones indicates CloneVM supports Linked=true.
	LinkedClones bool
	// MemorySnapshots indicates CreateSnapshot supports Memory=true.
	MemorySnapshots bool
	// QuiesceSnapshots indicates CreateSnapshot supports Quiesce=true.
	QuiesceSnapshots bool
	// LiveMigration indicates the provider can migrate running VMs.
	LiveMigration bool
	// GuestMetrics indicates GetVMMetrics returns real data (not zeros).
	GuestMetrics bool
	// TemplateClone indicates CreateVM supports TemplateID-based provisioning.
	TemplateClone bool
}

// ─────────────────────────────────────────────────────────────────────────────
// VM operations
// ─────────────────────────────────────────────────────────────────────────────

// VMProvider covers all virtual machine lifecycle operations.
type VMProvider interface {
	ListVMs(ctx context.Context) ([]VMInfo, error)
	GetVM(ctx context.Context, providerVMID string) (*VMInfo, error)
	CreateVM(ctx context.Context, spec VMCreateSpec) (*VMInfo, error)
	DeleteVM(ctx context.Context, providerVMID string) error
	CloneVM(ctx context.Context, providerVMID string, spec VMCloneSpec) (*VMInfo, error)

	PowerOn(ctx context.Context, providerVMID string) error
	PowerOff(ctx context.Context, providerVMID string) error
	Reboot(ctx context.Context, providerVMID string) error
	Suspend(ctx context.Context, providerVMID string) error
	Reset(ctx context.Context, providerVMID string) error

	GetVMMetrics(ctx context.Context, providerVMID string) (*VMMetrics, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Snapshot operations
// ─────────────────────────────────────────────────────────────────────────────

// SnapshotProvider covers snapshot lifecycle.
type SnapshotProvider interface {
	ListSnapshots(ctx context.Context, providerVMID string) ([]SnapshotInfo, error)
	CreateSnapshot(ctx context.Context, providerVMID string, spec SnapshotSpec) (*SnapshotInfo, error)
	DeleteSnapshot(ctx context.Context, providerVMID, snapshotID string) error
	RevertSnapshot(ctx context.Context, providerVMID, snapshotID string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Storage operations
// ─────────────────────────────────────────────────────────────────────────────

// StorageProvider covers datastore discovery.
type StorageProvider interface {
	ListDataStores(ctx context.Context) ([]DataStoreInfo, error)
	GetDataStore(ctx context.Context, id string) (*DataStoreInfo, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Network operations
// ─────────────────────────────────────────────────────────────────────────────

// NetworkProvider covers virtual network discovery.
type NetworkProvider interface {
	ListNetworks(ctx context.Context) ([]NetworkInfo, error)
	GetNetwork(ctx context.Context, id string) (*NetworkInfo, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Inventory sync
// ─────────────────────────────────────────────────────────────────────────────

// InventoryProvider covers full inventory synchronisation from the hypervisor.
// SyncInventory fetches the live state and returns a normalised snapshot that
// the service layer can reconcile against the database.
type InventoryProvider interface {
	SyncInventory(ctx context.Context) (*InventorySnapshot, error)
}

// TemplateProvider is an optional capability for providers that can discover
// VM templates separately from the main inventory sync.
// Check Capabilities().TemplateClone before casting.
type TemplateProvider interface {
	ListTemplates(ctx context.Context) ([]TemplateInfo, error)
}

// TemplateInfo is the normalised template representation returned by any provider.
type TemplateInfo struct {
	ProviderID  string
	Name        string
	Description string
	GuestOS     string
	CPUCount    int
	MemoryMB    int
	DiskGB      int
	Tags        []string
	Extra       map[string]interface{}
}

// ─────────────────────────────────────────────────────────────────────────────
// Data transfer objects (provider-agnostic)
// ─────────────────────────────────────────────────────────────────────────────

// VMInfo is the normalised VM representation returned by any provider.
type VMInfo struct {
	ProviderVMID string
	Name         string
	Status       model.VMStatus
	CPUCount     int
	MemoryMB     int
	DiskGB       int
	IPAddresses  []string
	MACAddress   string
	NetworkName  string
	GuestOS      string
	GuestOSType  string
	ToolsStatus  string
	Extra        map[string]interface{}
}

// VMCreateSpec defines the desired state for a new VM.
type VMCreateSpec struct {
	Name        string
	CPUCount    int
	MemoryMB    int
	DiskGB      int
	NetworkName string
	DataStore   string
	TemplateID  string
	GuestOS     string
	Extra       map[string]interface{}
}

// VMCloneSpec defines clone parameters.
type VMCloneSpec struct {
	Name      string
	DataStore string
	Linked    bool
	Extra     map[string]interface{}
}

// VMMetrics holds real-time performance counters.
type VMMetrics struct {
	CPUUsagePercent    float64
	MemoryUsagePercent float64
	DiskReadIOPS       float64
	DiskWriteIOPS      float64
	NetworkRxMbps      float64
	NetworkTxMbps      float64
}

// SnapshotInfo is the normalised snapshot representation.
type SnapshotInfo struct {
	ProviderID  string
	Name        string
	Description string
	IsCurrent   bool
	ParentID    string
	CreatedAt   time.Time
}

// SnapshotSpec defines snapshot creation parameters.
type SnapshotSpec struct {
	Name        string
	Description string
	Memory      bool
	Quiesce     bool
}

// DataStoreInfo is the normalised datastore representation.
type DataStoreInfo struct {
	ProviderID string
	Name       string
	Type       string
	CapacityGB float64
	UsedGB     float64
	FreeGB     float64
	Accessible bool
}

// NetworkInfo is the normalised network representation.
type NetworkInfo struct {
	ProviderID string
	Name       string
	Type       string
	VLAN       int
	Accessible bool
}

// InventorySnapshot is the full normalised state returned by SyncInventory.
// The service layer diffs this against the database to upsert/delete records.
type InventorySnapshot struct {
	VMs        []VMInfo
	DataStores []DataStoreInfo
	Networks   []NetworkInfo
	Hosts      []HostInfo
	Clusters   []ClusterInfo
	SyncedAt   time.Time
}

// HostInfo is the normalised host/node representation returned by any provider.
type HostInfo struct {
	ProviderID        string
	Name              string
	Status            string // connected/disconnected/maintenance/unknown
	ClusterProviderID string // empty if standalone
	CPUModel          string
	CPUSockets        int
	CPUCores          int
	CPUThreads        int
	CPUUsageMHz       int
	TotalMemoryMB     int
	UsedMemoryMB      int
	HypervisorVersion string
	UptimeSeconds     int64
	Extra             map[string]interface{}
}

// ClusterInfo is the normalised cluster representation returned by any provider.
type ClusterInfo struct {
	ProviderID    string
	Name          string
	TotalCPU      int
	TotalMemoryMB int
	HostCount     int
	Extra         map[string]interface{}
}

// ─────────────────────────────────────────────────────────────────────────────
// Console session
// ─────────────────────────────────────────────────────────────────────────────

// ConsoleType identifies the remote-access protocol.
type ConsoleType string

const (
	ConsoleTypeVNC    ConsoleType = "vnc"
	ConsoleTypeSpice  ConsoleType = "spice"
	ConsoleTypeWebMKS ConsoleType = "webmks" // VMware WebMKS
	ConsoleTypeNoVNC  ConsoleType = "novnc"  // Proxmox noVNC
	ConsoleTypeXTerm  ConsoleType = "xterm"  // serial/xterm.js
)

// ConsoleOptions controls how the console session is created.
type ConsoleOptions struct {
	// Type requests a specific protocol. Providers may fall back to their
	// default if the requested type is unavailable.
	Type ConsoleType
	// TTL is how long the session ticket should remain valid.
	// Zero means the provider default (typically 30–300 s).
	TTL time.Duration
}

// ConsoleSession carries the credentials needed to open a console connection.
// The caller is responsible for proxying or redirecting the end-user to URL.
type ConsoleSession struct {
	// Type is the protocol actually granted (may differ from requested).
	Type ConsoleType
	// URL is the full WebSocket or HTTP endpoint for the console.
	URL string
	// Ticket is a short-lived auth token embedded in URL or sent separately.
	Ticket string
	// Host / Port are set when the client must connect directly (non-URL flow).
	Host string
	Port int
	// ExpiresAt is when the ticket becomes invalid.
	ExpiresAt time.Time
	// Extra holds provider-specific fields (e.g. vmware thumbprint, proxmox node).
	Extra map[string]interface{}
}
