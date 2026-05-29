package port

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Generic pagination / filtering helpers
// ─────────────────────────────────────────────────────────────────────────────

// Page carries pagination parameters.
type Page struct {
	Number int // 1-based
	Size   int
}

// PageResult wraps a slice of results with total count metadata.
type PageResult[T any] struct {
	Items      []T
	TotalItems int64
	TotalPages int
	Page       int
	PageSize   int
}

// ErrNotFound is returned by repositories when a record does not exist.
var ErrNotFound = fmt.Errorf("record not found")

// ─────────────────────────────────────────────────────────────────────────────
// Repository interfaces
// ─────────────────────────────────────────────────────────────────────────────

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page Page) (*PageResult[model.User], error)
	// AssignRole adds a role to a user (idempotent).
	AssignRole(ctx context.Context, userID, roleID string) error
	// RevokeRole removes a role from a user.
	RevokeRole(ctx context.Context, userID, roleID string) error
	// GetPermissions returns all distinct permissions for a user across all their roles.
	GetPermissions(ctx context.Context, userID string) ([]model.Permission, error)
}

// RoleRepository defines persistence operations for roles.
type RoleRepository interface {
	Create(ctx context.Context, role *model.Role) error
	GetByID(ctx context.Context, id string) (*model.Role, error)
	GetByName(ctx context.Context, name string) (*model.Role, error)
	Update(ctx context.Context, role *model.Role) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.Role, error)
	// AssignPermission adds a permission to a role (idempotent).
	AssignPermission(ctx context.Context, roleID, permissionID string) error
	// RevokePermission removes a permission from a role.
	RevokePermission(ctx context.Context, roleID, permissionID string) error
}

// PermissionRepository defines persistence operations for permissions.
type PermissionRepository interface {
	Create(ctx context.Context, perm *model.Permission) error
	GetByID(ctx context.Context, id string) (*model.Permission, error)
	GetByResourceAction(ctx context.Context, resource, action string) (*model.Permission, error)
	List(ctx context.Context) ([]model.Permission, error)
	Delete(ctx context.Context, id string) error
}

// HypervisorRepository defines persistence operations for hypervisors.
type HypervisorRepository interface {
	Create(ctx context.Context, h *model.Hypervisor) error
	GetByID(ctx context.Context, id string) (*model.Hypervisor, error)
	Update(ctx context.Context, h *model.Hypervisor) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page Page) (*PageResult[model.Hypervisor], error)
	UpdateConnectionStatus(ctx context.Context, id string, status model.ConnectionStatus) error
}

// HypervisorGroupRepository defines persistence operations for hypervisor groups.
type HypervisorGroupRepository interface {
	Create(ctx context.Context, g *model.HypervisorGroup) error
	GetByID(ctx context.Context, id string) (*model.HypervisorGroup, error)
	Update(ctx context.Context, g *model.HypervisorGroup) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page Page) (*PageResult[model.HypervisorGroup], error)
}

// VMTemplateRepository defines persistence operations for VM templates.
type VMTemplateRepository interface {
	Create(ctx context.Context, t *model.VMTemplate) error
	GetByID(ctx context.Context, id string) (*model.VMTemplate, error)
	List(ctx context.Context, hypervisorID string, page Page) (*PageResult[model.VMTemplate], error)
	BulkUpsert(ctx context.Context, templates []model.VMTemplate) error
	Delete(ctx context.Context, id string) error
}

// VMRepository defines persistence operations for virtual machines.
type VMRepository interface {
	Create(ctx context.Context, vm *model.VM) error
	GetByID(ctx context.Context, id string) (*model.VM, error)
	GetByIDs(ctx context.Context, ids []string) ([]model.VM, error)
	GetByProviderID(ctx context.Context, hypervisorID, providerVMID string) (*model.VM, error)
	Update(ctx context.Context, vm *model.VM) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter VMFilter, page Page) (*PageResult[model.VM], error)
	UpdateStatus(ctx context.Context, id string, status model.VMStatus) error
	BulkUpsert(ctx context.Context, vms []model.VM) error
	// ListProviderIDs returns all provider_vm_id values for a hypervisor.
	// Used during incremental sync to detect deleted VMs.
	ListProviderIDs(ctx context.Context, hypervisorID string) ([]string, error)
	// MarkDeletedByProviderIDs soft-deletes VMs whose provider_vm_id is NOT in the given set.
	MarkDeletedByProviderIDs(ctx context.Context, hypervisorID string, activeIDs []string) (int64, error)
}

// VMFilter narrows VM list queries.
type VMFilter struct {
	HypervisorID string
	TagIDs       []string // filter to VMs that have ALL of these tags
	Status       string
}

// SnapshotRepository defines persistence operations for VM snapshots.
type SnapshotRepository interface {
	Create(ctx context.Context, s *model.Snapshot) error
	GetByID(ctx context.Context, id string) (*model.Snapshot, error)
	GetByProviderID(ctx context.Context, vmID, providerID string) (*model.Snapshot, error)
	ListByVMID(ctx context.Context, vmID string) ([]model.Snapshot, error)
	Delete(ctx context.Context, id string) error
	DeleteByProviderID(ctx context.Context, vmID, providerID string) error
	SetCurrentSnapshot(ctx context.Context, vmID, snapshotID string) error
	BulkUpsert(ctx context.Context, snaps []model.Snapshot) error
}

// DataStoreRepository defines persistence operations for datastores.
type DataStoreRepository interface {
	BulkUpsert(ctx context.Context, stores []model.DataStore) error
	List(ctx context.Context, hypervisorID string) ([]model.DataStore, error)
}

// NetworkRepository defines persistence operations for virtual networks.
type NetworkRepository interface {
	BulkUpsert(ctx context.Context, networks []model.Network) error
	List(ctx context.Context, hypervisorID string) ([]model.Network, error)
}

// TaskRepository defines persistence operations for async tasks.
type TaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	GetByID(ctx context.Context, id string) (*model.Task, error)
	Update(ctx context.Context, task *model.Task) error
	List(ctx context.Context, page Page) (*PageResult[model.Task], error)
	ListByVMID(ctx context.Context, vmID string, page Page) (*PageResult[model.Task], error)
	ListByParentID(ctx context.Context, parentID string) ([]model.Task, error)
	ListPending(ctx context.Context, limit int) ([]model.Task, error)
	UpdateStatus(ctx context.Context, id string, status model.TaskStatus, result model.JSONMap, errMsg string) error
	UpdateProgress(ctx context.Context, id string, progress int) error
	AppendLog(ctx context.Context, entry *model.TaskLog) error
	GetLogs(ctx context.Context, taskID string, page Page) (*PageResult[model.TaskLog], error)
}

// AuditRepository defines persistence operations for audit logs.
type AuditRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, filter AuditFilter, page Page) (*PageResult[model.AuditLog], error)
}

// AuditFilter narrows audit log queries.
type AuditFilter struct {
	UserID       *uuid.UUID
	HypervisorID *uuid.UUID
	Resource     string
	ResourceID   *uuid.UUID
	Action       model.AuditAction
	Since        *time.Time
	Until        *time.Time
	SuccessOnly  *bool
	Search       string // partial match on username or description
}

// WebSocketEventRepository defines persistence for domain events.
type WebSocketEventRepository interface {
	Create(ctx context.Context, event *model.WebSocketEvent) error
	// ListSince returns events for a room created after the given cursor,
	// ordered oldest-first. Used for client catch-up on reconnect.
	ListSince(ctx context.Context, room string, since time.Time, limit int) ([]model.WebSocketEvent, error)
	// MarkDelivered stamps delivered_at on a batch of event IDs.
	MarkDelivered(ctx context.Context, ids []uuid.UUID) error
}

// ConsoleSessionRepository defines persistence operations for console sessions.
type ConsoleSessionRepository interface {
	Create(ctx context.Context, s *model.ConsoleSession) error
	GetByToken(ctx context.Context, token string) (*model.ConsoleSession, error)
	GetByID(ctx context.Context, id string) (*model.ConsoleSession, error)
	UpdateStatus(ctx context.Context, id string, status model.ConsoleSessionStatus) error
	ExpireOld(ctx context.Context) (int64, error)
}

// ProviderHealthRepository defines persistence operations for provider health snapshots.
type ProviderHealthRepository interface {
	// Upsert inserts or updates the health snapshot for a hypervisor.
	Upsert(ctx context.Context, h *model.ProviderHealth) error
	// GetByHypervisorID returns the current health snapshot for a hypervisor.
	GetByHypervisorID(ctx context.Context, hypervisorID string) (*model.ProviderHealth, error)
	// List returns health snapshots for all hypervisors.
	List(ctx context.Context) ([]model.ProviderHealth, error)
	// AppendHistory inserts a history row.
	AppendHistory(ctx context.Context, h *model.ProviderHealthHistory) error
	// GetHistory returns the last N history rows for a hypervisor, newest-first.
	GetHistory(ctx context.Context, hypervisorID string, limit int) ([]model.ProviderHealthHistory, error)
}

// PlatformEventRepository defines persistence for platform-level events.
type PlatformEventRepository interface {
	Create(ctx context.Context, event *model.PlatformEvent) error
	GetByID(ctx context.Context, id string) (*model.PlatformEvent, error)
	List(ctx context.Context, filter PlatformEventFilter, page Page) (*PageResult[model.PlatformEvent], error)
}

// PlatformEventFilter narrows platform event queries.
type PlatformEventFilter struct {
	EventType    string
	Severity     string
	Provider     string
	ResourceType string
	ResourceID   *uuid.UUID
	HypervisorID *uuid.UUID
	Since        *time.Time
	Until        *time.Time
	Search       string
}

// NotificationChannelRepository defines persistence for notification channels.
type NotificationChannelRepository interface {
	Create(ctx context.Context, ch *model.NotificationChannel) error
	GetByID(ctx context.Context, id string) (*model.NotificationChannel, error)
	Update(ctx context.Context, ch *model.NotificationChannel) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.NotificationChannel, error)
}

// NotificationRuleRepository defines persistence for notification rules.
type NotificationRuleRepository interface {
	Create(ctx context.Context, rule *model.NotificationRule) error
	GetByID(ctx context.Context, id string) (*model.NotificationRule, error)
	Update(ctx context.Context, rule *model.NotificationRule) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.NotificationRule, error)
	// ListEnabled returns all enabled rules with their channels preloaded.
	ListEnabled(ctx context.Context) ([]model.NotificationRule, error)
	// UpdateLastTriggered stamps last_triggered_at for throttle tracking.
	UpdateLastTriggered(ctx context.Context, id string) error
}

// NotificationHistoryRepository defines persistence for delivery history.
type NotificationHistoryRepository interface {
	Create(ctx context.Context, h *model.NotificationHistory) error
	List(ctx context.Context, filter NotificationHistoryFilter, page Page) (*PageResult[model.NotificationHistory], error)
}

// NotificationHistoryFilter narrows notification history queries.
type NotificationHistoryFilter struct {
	RuleID    *uuid.UUID
	ChannelID *uuid.UUID
	EventID   *uuid.UUID
	Status    string
	Since     *time.Time
	Until     *time.Time
}

// ScheduleRepository defines persistence operations for schedules.
type ScheduleRepository interface {
	Create(ctx context.Context, s *model.Schedule) error
	GetByID(ctx context.Context, id string) (*model.Schedule, error)
	Update(ctx context.Context, s *model.Schedule) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ScheduleFilter, page Page) (*PageResult[model.Schedule], error)
	// ListDue returns enabled schedules whose next_run_at is <= now.
	ListDue(ctx context.Context, now time.Time) ([]model.Schedule, error)
	// UpdateAfterRun updates last_run_at, next_run_at, run_count, failure_count, last_task_id, last_run_status.
	UpdateAfterRun(ctx context.Context, id string, update ScheduleRunUpdate) error
}

// ScheduleFilter narrows schedule list queries.
type ScheduleFilter struct {
	OperationType string
	TargetType    string
	Enabled       *bool
	Status        string
}

// ScheduleRunUpdate carries the fields updated after a schedule fires.
type ScheduleRunUpdate struct {
	LastRunAt     time.Time
	NextRunAt     *time.Time
	LastTaskID    *uuid.UUID
	LastRunStatus string
	RunCount      int
	FailureCount  int
	Status        model.ScheduleStatus
}

// ScheduleExecutionRepository defines persistence for schedule execution history.
type ScheduleExecutionRepository interface {
	Create(ctx context.Context, e *model.ScheduleExecution) error
	List(ctx context.Context, scheduleID string, page Page) (*PageResult[model.ScheduleExecution], error)
}

// WorkflowRepository defines persistence operations for automation workflows.
type WorkflowRepository interface {
	Create(ctx context.Context, w *model.Workflow) error
	GetByID(ctx context.Context, id string) (*model.Workflow, error)
	Update(ctx context.Context, w *model.Workflow) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter WorkflowFilter, page Page) (*PageResult[model.Workflow], error)
	// ListByTrigger returns enabled workflows for a given trigger type.
	ListByTrigger(ctx context.Context, triggerType model.WorkflowTriggerType) ([]model.Workflow, error)
	// UpdateAfterRun updates run_count, failure_count, last_run_at, last_run_status.
	UpdateAfterRun(ctx context.Context, id string, update WorkflowRunUpdate) error
}

// WorkflowFilter narrows workflow list queries.
type WorkflowFilter struct {
	TriggerType string
	Enabled     *bool
	Status      string
}

// WorkflowRunUpdate carries the fields updated after a workflow run.
type WorkflowRunUpdate struct {
	LastRunAt     time.Time
	LastRunStatus string
	RunCount      int
	FailureCount  int
}

// WorkflowRunRepository defines persistence for workflow run history.
type WorkflowRunRepository interface {
	Create(ctx context.Context, r *model.WorkflowRun) error
	GetByID(ctx context.Context, id string) (*model.WorkflowRun, error)
	Update(ctx context.Context, r *model.WorkflowRun) error
	List(ctx context.Context, workflowID string, page Page) (*PageResult[model.WorkflowRun], error)
	// CountActive returns the number of running/pending runs for a workflow.
	CountActive(ctx context.Context, workflowID string) (int64, error)
}

// TagRepository defines persistence operations for VM tags.
type TagRepository interface {
	Create(ctx context.Context, tag *model.Tag) error
	GetByID(ctx context.Context, id string) (*model.Tag, error)
	GetByName(ctx context.Context, name string) (*model.Tag, error)
	Update(ctx context.Context, tag *model.Tag) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]model.Tag, error)
	// AddToVM attaches a tag to a VM (idempotent).
	AddToVM(ctx context.Context, vmID, tagID string) error
	// RemoveFromVM detaches a tag from a VM.
	RemoveFromVM(ctx context.Context, vmID, tagID string) error
	// ListByVM returns all tags attached to a VM.
	ListByVM(ctx context.Context, vmID string) ([]model.Tag, error)
	// ListVMsByTag returns VM IDs that have the given tag.
	ListVMsByTag(ctx context.Context, tagID string) ([]string, error)
}

// ProvisioningJobRepository defines persistence operations for provisioning jobs.
type ProvisioningJobRepository interface {
	Create(ctx context.Context, job *model.ProvisioningJob) error
	GetByID(ctx context.Context, id string) (*model.ProvisioningJob, error)
	Update(ctx context.Context, job *model.ProvisioningJob) error
	List(ctx context.Context, filter ProvisioningJobFilter, page Page) (*PageResult[model.ProvisioningJob], error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Infrastructure hierarchy repositories
// ─────────────────────────────────────────────────────────────────────────────

// ClusterRepository defines persistence operations for compute clusters.
type ClusterRepository interface {
	BulkUpsert(ctx context.Context, clusters []model.Cluster) error
	List(ctx context.Context, hypervisorID string) ([]model.Cluster, error)
	GetByID(ctx context.Context, id string) (*model.Cluster, error)
	GetByProviderID(ctx context.Context, hypervisorID, providerID string) (*model.Cluster, error)
}

// HostRepository defines persistence operations for hypervisor hosts/nodes.
type HostRepository interface {
	BulkUpsert(ctx context.Context, hosts []model.Host) error
	List(ctx context.Context, hypervisorID string) ([]model.Host, error)
	ListByCluster(ctx context.Context, clusterID string) ([]model.Host, error)
	GetByID(ctx context.Context, id string) (*model.Host, error)
	GetByProviderID(ctx context.Context, hypervisorID, providerID string) (*model.Host, error)
	UpdateVMCount(ctx context.Context, hypervisorID string) error
}
