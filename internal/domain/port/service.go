package port

import (
	"context"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Service interfaces — consumed by HTTP handlers and the task engine
// ─────────────────────────────────────────────────────────────────────────────

// AuthService handles authentication and token management.
type AuthService interface {
	Login(ctx context.Context, email, password string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error)
	ValidateAccessToken(ctx context.Context, token string) (*Claims, error)
}

// TokenPair holds an access/refresh token pair.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Claims carries the decoded JWT payload.
type Claims struct {
	UserID      string
	Username    string
	Roles       []string
	Permissions []string // "resource:action" strings for fast middleware checks
}

// UserService handles user management.
type UserService interface {
	Create(ctx context.Context, req CreateUserRequest) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
	Update(ctx context.Context, id string, req UpdateUserRequest) (*model.User, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page Page) (*PageResult[model.User], error)
	AssignRole(ctx context.Context, userID, roleID string) error
	RevokeRole(ctx context.Context, userID, roleID string) error
	// GetPermissions returns all distinct permissions for a user across all their roles.
	GetPermissions(ctx context.Context, userID string) ([]model.Permission, error)
	// ChangePassword updates a user's password after verifying the current one.
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
}

// CreateUserRequest carries new-user data.
type CreateUserRequest struct {
	Email     string
	Username  string
	Password  string
	FirstName string
	LastName  string
	RoleIDs   []string
}

// UpdateUserRequest carries user update data.
type UpdateUserRequest struct {
	FirstName *string
	LastName  *string
	IsActive  *bool
}

// RoleService handles role and permission management.
type RoleService interface {
	Create(ctx context.Context, req CreateRoleRequest) (*model.Role, error)
	GetByID(ctx context.Context, id string) (*model.Role, error)
	Update(ctx context.Context, id string, req UpdateRoleRequest) (*model.Role, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.Role, error)
	AssignPermission(ctx context.Context, roleID, permissionID string) error
	RevokePermission(ctx context.Context, roleID, permissionID string) error
	// SetPermissions replaces all permissions on a role with the given set.
	SetPermissions(ctx context.Context, roleID string, permissionIDs []string) error
}

// CreateRoleRequest carries new-role data.
type CreateRoleRequest struct {
	Name          string
	Description   string
	PermissionIDs []string
}

// UpdateRoleRequest carries role update data.
type UpdateRoleRequest struct {
	Name        *string
	Description *string
}

// PermissionService handles permission management.
type PermissionService interface {
	Create(ctx context.Context, req CreatePermissionRequest) (*model.Permission, error)
	GetByID(ctx context.Context, id string) (*model.Permission, error)
	List(ctx context.Context) ([]model.Permission, error)
	Delete(ctx context.Context, id string) error
}

// CreatePermissionRequest carries new-permission data.
type CreatePermissionRequest struct {
	Resource string
	Action   string
}

// HasPermission checks whether a set of role names grants a resource/action.
// This is a pure helper used by the permission middleware.
func HasPermission(roles []string, resource, action string, permissions []model.Permission) bool {
	for _, p := range permissions {
		if p.Resource == resource && p.Action == action {
			return true
		}
	}
	return false
}

// SyncResult summarises the outcome of an inventory synchronisation.
type SyncResult struct {
	HypervisorID string
	VMsAdded     int
	VMsUpdated   int
	VMsRemoved   int
	StoresUpserted int
	NetworksUpserted int
	Errors       []string
}

// HypervisorService manages hypervisor registration and connectivity.
type HypervisorService interface {
	Register(ctx context.Context, req RegisterHypervisorRequest) (*model.Hypervisor, error)
	GetByID(ctx context.Context, id string) (*model.Hypervisor, error)
	Update(ctx context.Context, id string, req UpdateHypervisorRequest) (*model.Hypervisor, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page Page) (*PageResult[model.Hypervisor], error)
	TestConnection(ctx context.Context, id string) error
	// SyncInventory creates an async task and returns its ID (HTTP 202 path).
	SyncInventory(ctx context.Context, id string) (string, error)
	// SyncInventoryNow performs the full sync synchronously. Called by the task engine.
	SyncInventoryNow(ctx context.Context, id string, progress func(pct int, msg string)) (*SyncResult, error)
	// BuildCredentials decrypts and returns the connection credentials for a hypervisor.
	// Used by the task engine to connect to the provider for VM operations.
	BuildCredentials(ctx context.Context, id string) (Credentials, model.ProviderType, error)
}

// RegisterHypervisorRequest carries hypervisor registration data.
type RegisterHypervisorRequest struct {
	Name        string
	Description string
	Provider    model.ProviderType
	Host        string
	Port        int
	Username    string
	Password    string
	Token       string
	TLSVerify   bool
	Tags        []string
	// Provider-specific metadata stored in the Metadata JSONB column.
	// VMware: vcenter_url, datacenter
	// Proxmox: node, api_token_id, api_token_secret
	Metadata model.JSONMap
}

// UpdateHypervisorRequest carries hypervisor update data.
type UpdateHypervisorRequest struct {
	Name        *string
	Description *string
	Host        *string
	Port        *int
	Username    *string
	Password    *string
	Token       *string
	TLSVerify   *bool
	Tags        []string
	Metadata    model.JSONMap
}

// VMService manages virtual machine operations.
type VMService interface {
	GetByID(ctx context.Context, id string) (*model.VM, error)
	List(ctx context.Context, filter VMFilter, page Page) (*PageResult[model.VM], error)
	Delete(ctx context.Context, vmID string) error
	PowerOn(ctx context.Context, vmID string) (string, error)  // returns task ID
	PowerOff(ctx context.Context, vmID string) (string, error)
	Reboot(ctx context.Context, vmID string) (string, error)
	Suspend(ctx context.Context, vmID string) (string, error)
	ListSnapshots(ctx context.Context, vmID string) ([]model.Snapshot, error)
	CreateSnapshot(ctx context.Context, vmID string, spec SnapshotSpec) (string, error)
	RevertSnapshot(ctx context.Context, vmID, snapshotID string) (string, error)
	DeleteSnapshot(ctx context.Context, vmID, snapshotID string) (string, error)
	GetMetrics(ctx context.Context, vmID string) (*VMMetrics, error)
	// Bulk operations — fan out individual tasks and return a parent task ID.
	BulkPowerOn(ctx context.Context, vmIDs []string) (string, error)
	BulkPowerOff(ctx context.Context, vmIDs []string) (string, error)
	BulkReboot(ctx context.Context, vmIDs []string) (string, error)
	BulkSnapshot(ctx context.Context, vmIDs []string, spec SnapshotSpec) (string, error)
}

// TagService manages VM tag lifecycle.
type TagService interface {
	Create(ctx context.Context, req CreateTagRequest) (*model.Tag, error)
	GetByID(ctx context.Context, id string) (*model.Tag, error)
	Update(ctx context.Context, id string, req UpdateTagRequest) (*model.Tag, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.Tag, error)
	AddToVM(ctx context.Context, vmID, tagID string) error
	RemoveFromVM(ctx context.Context, vmID, tagID string) error
	ListByVM(ctx context.Context, vmID string) ([]model.Tag, error)
	// ListVMsByTag returns VM IDs that have the given tag.
	ListVMsByTag(ctx context.Context, tagID string) ([]string, error)
}

// CreateTagRequest carries new-tag data.
type CreateTagRequest struct {
	Name        string
	Color       string
	Description string
}

// UpdateTagRequest carries tag update data.
type UpdateTagRequest struct {
	Name        *string
	Color       *string
	Description *string
}

// TaskService manages async task lifecycle.
type TaskService interface {
	GetByID(ctx context.Context, id string) (*model.Task, error)
	List(ctx context.Context, page Page) (*PageResult[model.Task], error)
	ListByVMID(ctx context.Context, vmID string, page Page) (*PageResult[model.Task], error)
	Cancel(ctx context.Context, id string) error
	GetLogs(ctx context.Context, taskID string, page Page) (*PageResult[model.TaskLog], error)
}

// ConsoleService manages console session lifecycle.
type ConsoleService interface {
	// RequestSession acquires a provider console ticket, persists a session record,
	// and returns the session (including the browser-openable URL).
	RequestSession(ctx context.Context, vmID string, opts ConsoleOptions) (*model.ConsoleSession, error)
	// GetSession looks up an active session by its opaque token.
	// Returns an error if the session does not exist or has expired.
	GetSession(ctx context.Context, token string) (*model.ConsoleSession, error)
}

// AuditService records and queries audit events.
type AuditService interface {
	Log(ctx context.Context, entry AuditEntry) error
	List(ctx context.Context, filter AuditFilter, page Page) (*PageResult[model.AuditLog], error)
}

// AuditEntry is the input to AuditService.Log.
type AuditEntry struct {
	UserID      string
	Username    string
	Action      model.AuditAction
	Resource    string
	ResourceID  string
	Description string
	IPAddress   string
	UserAgent   string
	RequestID   string
	Changes     model.JSONMap
	Success     bool
	ErrorMsg    string
}

// ─────────────────────────────────────────────────────────────────────────────
// Event & Notification services
// ─────────────────────────────────────────────────────────────────────────────

// EventDispatchRequest carries the data needed to emit a platform event.
type EventDispatchRequest struct {
	EventType    model.PlatformEventType
	Severity     model.PlatformEventSeverity // optional — defaults to SeverityForEventType
	Provider     string
	ResourceType string
	ResourceID   string
	HypervisorID string
	Message      string
	Metadata     model.JSONMap
}

// PlatformEventService manages platform event persistence and querying.
type PlatformEventService interface {
	// Dispatch persists a platform event, publishes it to the event bus,
	// and triggers notification delivery. Fire-and-forget safe.
	Dispatch(ctx context.Context, req EventDispatchRequest) error
	// List returns paginated platform events with optional filters.
	List(ctx context.Context, filter PlatformEventFilter, page Page) (*PageResult[model.PlatformEvent], error)
	// GetByID returns a single platform event.
	GetByID(ctx context.Context, id string) (*model.PlatformEvent, error)
}

// NotificationChannelService manages notification channel CRUD.
type NotificationChannelService interface {
	Create(ctx context.Context, req CreateNotificationChannelRequest) (*model.NotificationChannel, error)
	GetByID(ctx context.Context, id string) (*model.NotificationChannel, error)
	Update(ctx context.Context, id string, req UpdateNotificationChannelRequest) (*model.NotificationChannel, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.NotificationChannel, error)
	// Test sends a test notification to the channel.
	Test(ctx context.Context, id string) error
}

// CreateNotificationChannelRequest carries new-channel data.
type CreateNotificationChannelRequest struct {
	Name        string
	Type        model.NotificationChannelType
	Description string
	Enabled     bool
	Config      model.JSONMap
}

// UpdateNotificationChannelRequest carries channel update data.
type UpdateNotificationChannelRequest struct {
	Name        *string
	Description *string
	Enabled     *bool
	Config      model.JSONMap
}

// NotificationRuleService manages notification rule CRUD.
type NotificationRuleService interface {
	Create(ctx context.Context, req CreateNotificationRuleRequest) (*model.NotificationRule, error)
	GetByID(ctx context.Context, id string) (*model.NotificationRule, error)
	Update(ctx context.Context, id string, req UpdateNotificationRuleRequest) (*model.NotificationRule, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.NotificationRule, error)
}

// CreateNotificationRuleRequest carries new-rule data.
type CreateNotificationRuleRequest struct {
	Name            string
	Description     string
	ChannelID       string
	EventTypes      []string
	Severities      []string
	Providers       []string
	ThrottleSeconds int
	Enabled         bool
}

// UpdateNotificationRuleRequest carries rule update data.
type UpdateNotificationRuleRequest struct {
	Name            *string
	Description     *string
	ChannelID       *string
	EventTypes      []string
	Severities      []string
	Providers       []string
	ThrottleSeconds *int
	Enabled         *bool
}

// NotificationHistoryService manages delivery history queries.
type NotificationHistoryService interface {
	List(ctx context.Context, filter NotificationHistoryFilter, page Page) (*PageResult[model.NotificationHistory], error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Scheduler & Automation services
// ─────────────────────────────────────────────────────────────────────────────

// ScheduleService manages schedule CRUD and execution.
type ScheduleService interface {
	Create(ctx context.Context, req CreateScheduleRequest) (*model.Schedule, error)
	GetByID(ctx context.Context, id string) (*model.Schedule, error)
	Update(ctx context.Context, id string, req UpdateScheduleRequest) (*model.Schedule, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ScheduleFilter, page Page) (*PageResult[model.Schedule], error)
	// Enable/Disable toggle the enabled flag.
	Enable(ctx context.Context, id string) error
	Disable(ctx context.Context, id string) error
	// TriggerNow fires the schedule immediately (bypasses cron timing).
	TriggerNow(ctx context.Context, id string) (string, error)
	// ListExecutions returns execution history for a schedule.
	ListExecutions(ctx context.Context, scheduleID string, page Page) (*PageResult[model.ScheduleExecution], error)
}

// CreateScheduleRequest carries new-schedule data.
type CreateScheduleRequest struct {
	Name           string
	Description    string
	OperationType  model.ScheduleOperationType
	TargetType     model.ScheduleTargetType
	TargetIDs      []string
	ScheduleType   model.ScheduleType
	CronExpression string
	Timezone       string
	Enabled        bool
	MaxRuns        int
	ExpiresAt      *time.Time
	Payload        model.JSONMap
}

// UpdateScheduleRequest carries schedule update data.
type UpdateScheduleRequest struct {
	Name           *string
	Description    *string
	OperationType  *model.ScheduleOperationType
	TargetType     *model.ScheduleTargetType
	TargetIDs      []string
	ScheduleType   *model.ScheduleType
	CronExpression *string
	Timezone       *string
	Enabled        *bool
	MaxRuns        *int
	ExpiresAt      *time.Time
	Payload        model.JSONMap
}

// WorkflowService manages automation workflow CRUD and execution.
type WorkflowService interface {
	Create(ctx context.Context, req CreateWorkflowRequest) (*model.Workflow, error)
	GetByID(ctx context.Context, id string) (*model.Workflow, error)
	Update(ctx context.Context, id string, req UpdateWorkflowRequest) (*model.Workflow, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter WorkflowFilter, page Page) (*PageResult[model.Workflow], error)
	// Enable/Disable toggle the enabled flag.
	Enable(ctx context.Context, id string) error
	Disable(ctx context.Context, id string) error
	// TriggerNow fires the workflow immediately with optional trigger data.
	TriggerNow(ctx context.Context, id string, triggerData model.JSONMap) (string, error)
	// ListRuns returns execution history for a workflow.
	ListRuns(ctx context.Context, workflowID string, page Page) (*PageResult[model.WorkflowRun], error)
	// GetRun returns a single workflow run.
	GetRun(ctx context.Context, runID string) (*model.WorkflowRun, error)
}

// CreateWorkflowRequest carries new-workflow data.
type CreateWorkflowRequest struct {
	Name              string
	Description       string
	Enabled           bool
	TriggerType       model.WorkflowTriggerType
	TriggerConfig     model.JSONMap
	Conditions        model.JSONMap
	ContinueOnError   bool
	MaxConcurrentRuns int
	Actions           []CreateWorkflowActionRequest
}

// CreateWorkflowActionRequest carries a single action definition.
type CreateWorkflowActionRequest struct {
	Order          int
	ActionType     model.WorkflowActionType
	Name           string
	Description    string
	Config         model.JSONMap
	RetryCount     int
	TimeoutSeconds int
	ContinueOnError *bool
}

// UpdateWorkflowRequest carries workflow update data.
type UpdateWorkflowRequest struct {
	Name              *string
	Description       *string
	Enabled           *bool
	TriggerType       *model.WorkflowTriggerType
	TriggerConfig     model.JSONMap
	Conditions        model.JSONMap
	ContinueOnError   *bool
	MaxConcurrentRuns *int
	Actions           []CreateWorkflowActionRequest // nil = no change; non-nil = replace all
}

// ProviderHealthService manages provider health monitoring.
type ProviderHealthService interface {
	// GetAll returns the current health snapshot for all hypervisors.
	GetAll(ctx context.Context) ([]model.ProviderHealth, error)
	// GetByHypervisorID returns the current health snapshot for a single hypervisor.
	GetByHypervisorID(ctx context.Context, hypervisorID string) (*model.ProviderHealth, error)
	// GetHistory returns the last N latency/health history points for a hypervisor.
	GetHistory(ctx context.Context, hypervisorID string, limit int) ([]model.ProviderHealthHistory, error)
	// RunCheck performs an immediate health check for a single hypervisor and persists the result.
	RunCheck(ctx context.Context, hypervisorID string) (*model.ProviderHealth, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Template & Provisioning services
// ─────────────────────────────────────────────────────────────────────────────

// TemplateService manages VM template discovery and listing.
type TemplateService interface {
	// List returns paginated templates, optionally filtered by hypervisor.
	List(ctx context.Context, hypervisorID string, page Page) (*PageResult[model.VMTemplate], error)
	// GetByID returns a single template.
	GetByID(ctx context.Context, id string) (*model.VMTemplate, error)
	// SyncTemplates discovers templates from the provider and persists them.
	// Returns the task ID for async tracking.
	SyncTemplates(ctx context.Context, hypervisorID string) (string, error)
	// SyncTemplatesNow performs the sync synchronously (called by the task engine).
	SyncTemplatesNow(ctx context.Context, hypervisorID string, progress func(pct int, msg string)) (int, error)
}

// ProvisioningService manages VM clone and provision operations.
type ProvisioningService interface {
	// Clone creates a clone of an existing VM. Returns the provisioning job ID and task ID.
	Clone(ctx context.Context, req CloneVMRequest) (*model.ProvisioningJob, error)
	// Provision creates a new VM from a template. Returns the provisioning job.
	Provision(ctx context.Context, req ProvisionVMRequest) (*model.ProvisioningJob, error)
	// GetJob returns a provisioning job by ID.
	GetJob(ctx context.Context, id string) (*model.ProvisioningJob, error)
	// ListJobs returns paginated provisioning jobs.
	ListJobs(ctx context.Context, filter ProvisioningJobFilter, page Page) (*PageResult[model.ProvisioningJob], error)
}

// CloneVMRequest carries parameters for a VM clone operation.
type CloneVMRequest struct {
	SourceVMID  string
	Name        string
	DataStore   string
	Node        string
	Linked      bool
	Tags        []string
	Metadata    model.JSONMap
}

// ProvisionVMRequest carries parameters for provisioning a VM from a template.
type ProvisionVMRequest struct {
	TemplateID  string
	Name        string
	CPUCount    int
	MemoryMB    int
	DiskGB      int
	NetworkName string
	DataStore   string
	Node        string
	Tags        []string
	Metadata    model.JSONMap
}

// ProvisioningJobFilter narrows provisioning job list queries.
type ProvisioningJobFilter struct {
	HypervisorID string
	Type         string
	Status       string
}

// ─────────────────────────────────────────────────────────────────────────────
// Infrastructure hierarchy service
// ─────────────────────────────────────────────────────────────────────────────

// InfrastructureTreeNode is a node in the infrastructure hierarchy tree.
type InfrastructureTreeNode struct {
	ID           string                    `json:"id"`
	Type         string                    `json:"type"` // "provider" | "cluster" | "host" | "vm"
	Name         string                    `json:"name"`
	Status       string                    `json:"status"`
	ProviderType string                    `json:"provider_type,omitempty"`
	VMCount      int                       `json:"vm_count"`
	Children     []*InfrastructureTreeNode `json:"children,omitempty"`
	Metadata     map[string]interface{}    `json:"metadata,omitempty"`
}

// InfrastructureService provides infrastructure hierarchy queries.
type InfrastructureService interface {
	GetTree(ctx context.Context, hypervisorID string) ([]*InfrastructureTreeNode, error)
	ListHosts(ctx context.Context, hypervisorID string) ([]model.Host, error)
	GetHost(ctx context.Context, id string) (*HostDetail, error)
	ListClusters(ctx context.Context, hypervisorID string) ([]model.Cluster, error)
	GetCluster(ctx context.Context, id string) (*model.Cluster, error)
	ListDataStores(ctx context.Context, hypervisorID string) ([]model.DataStore, error)
	ListNetworks(ctx context.Context, hypervisorID string) ([]model.Network, error)
	// New aggregation endpoints
	GetTopology(ctx context.Context, hypervisorID string) (*TopologyGraph, error)
	GetHeatmap(ctx context.Context, hypervisorID string) (*InfraHeatmap, error)
	GetSummary(ctx context.Context) (*InfraSummary, error)
}

// HostDetail is the full host detail response including hosted VMs and datastores.
type HostDetail struct {
	Host       model.Host        `json:"host"`
	VMs        []model.VM        `json:"vms"`
	DataStores []model.DataStore `json:"datastores"`
	Networks   []model.Network   `json:"networks"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Topology graph types
// ─────────────────────────────────────────────────────────────────────────────

// TopologyNode represents a node in the infrastructure topology graph.
type TopologyNode struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`   // provider|cluster|host|datastore|network|vm
	Label        string                 `json:"label"`
	Status       string                 `json:"status"` // healthy|warning|critical|unknown
	ProviderType string                 `json:"provider_type,omitempty"`
	VMCount      int                    `json:"vm_count,omitempty"`
	CPUUsagePct  float64                `json:"cpu_usage_pct,omitempty"`
	MemUsagePct  float64                `json:"mem_usage_pct,omitempty"`
	DiskUsagePct float64                `json:"disk_usage_pct,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TopologyEdge represents a relationship between two topology nodes.
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // contains|runs_on|uses|connected_to
}

// TopologyGraph is the full infrastructure topology for visualization.
type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Heatmap types
// ─────────────────────────────────────────────────────────────────────────────

// HeatmapCell represents a single cell in a resource heatmap.
type HeatmapCell struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	HypervisorID string  `json:"hypervisor_id"`
	Value        float64 `json:"value"`        // 0–100 percentage
	Status       string  `json:"status"`       // healthy|warning|critical
	VMCount      int     `json:"vm_count,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// InfraHeatmap contains heatmap data for all resource dimensions.
type InfraHeatmap struct {
	CPU        []HeatmapCell `json:"cpu"`
	Memory     []HeatmapCell `json:"memory"`
	Datastore  []HeatmapCell `json:"datastore"`
	VMDensity  []HeatmapCell `json:"vm_density"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Infrastructure summary types
// ─────────────────────────────────────────────────────────────────────────────

// ProviderSummary is a per-hypervisor summary for the operations dashboard.
type ProviderSummary struct {
	HypervisorID    string     `json:"hypervisor_id"`
	HypervisorName  string     `json:"hypervisor_name"`
	Provider        string     `json:"provider"`
	Status          string     `json:"status"`
	VMCount         int        `json:"vm_count"`
	RunningVMs      int        `json:"running_vms"`
	HostCount       int        `json:"host_count"`
	ClusterCount    int        `json:"cluster_count"`
	CPUUsagePct     float64    `json:"cpu_usage_pct"`
	MemUsagePct     float64    `json:"mem_usage_pct"`
	StorageUsagePct float64    `json:"storage_usage_pct"`
	OverloadedHosts int        `json:"overloaded_hosts"`
	StaleSnapshots  int        `json:"stale_snapshots"`
	FailedTasks24h  int        `json:"failed_tasks_24h"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
}

// OperationsAlert is a single operational alert for the live dashboard.
type OperationsAlert struct {
	ID           string `json:"id"`
	Severity     string `json:"severity"` // critical|warning|info
	Category     string `json:"category"` // provider|host|task|snapshot|sync|capacity
	Title        string `json:"title"`
	Description  string `json:"description"`
	ResourceID   string `json:"resource_id,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	HypervisorID string `json:"hypervisor_id,omitempty"`
}

// InfraSummary is the full operations dashboard summary.
type InfraSummary struct {
	// Counts
	TotalProviders      int `json:"total_providers"`
	HealthyProviders    int `json:"healthy_providers"`
	UnhealthyProviders  int `json:"unhealthy_providers"`
	TotalHosts          int `json:"total_hosts"`
	ConnectedHosts      int `json:"connected_hosts"`
	DisconnectedHosts   int `json:"disconnected_hosts"`
	TotalVMs            int `json:"total_vms"`
	RunningVMs          int `json:"running_vms"`
	TotalClusters       int `json:"total_clusters"`
	TotalDatastores     int `json:"total_datastores"`
	TotalNetworks       int `json:"total_networks"`
	// Task stats
	ActiveTasks         int `json:"active_tasks"`
	FailedTasks24h      int `json:"failed_tasks_24h"`
	// Alerts
	CriticalAlerts      int `json:"critical_alerts"`
	WarningAlerts       int `json:"warning_alerts"`
	// Per-provider breakdown
	Providers           []ProviderSummary  `json:"providers"`
	// Operational alerts
	Alerts              []OperationsAlert  `json:"alerts"`
	// Relationship counts
	VMHostRelationships int `json:"vm_host_relationships"`
	StaleSnapshots      int `json:"stale_snapshots"`
	OverloadedHosts     int `json:"overloaded_hosts"`
}
