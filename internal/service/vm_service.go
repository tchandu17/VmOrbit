package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/pkg/logger"
)

type vmService struct {
	vms         port.VMRepository
	hypervisors port.HypervisorRepository
	tasks       port.TaskRepository
	snapshots   port.SnapshotRepository
	registry    *provider.Registry
	audit       port.AuditService
	log         logger.Logger
}

// NewVMService creates a new VM service.
func NewVMService(
	vms port.VMRepository,
	hypervisors port.HypervisorRepository,
	tasks port.TaskRepository,
	snapshots port.SnapshotRepository,
	registry *provider.Registry,
	audit port.AuditService,
	log logger.Logger,
) port.VMService {
	return &vmService{
		vms:         vms,
		hypervisors: hypervisors,
		tasks:       tasks,
		snapshots:   snapshots,
		registry:    registry,
		audit:       audit,
		log:         log,
	}
}

func (s *vmService) GetByID(ctx context.Context, id string) (*model.VM, error) {
	return s.vms.GetByID(ctx, id)
}

func (s *vmService) List(ctx context.Context, filter port.VMFilter, page port.Page) (*port.PageResult[model.VM], error) {
	return s.vms.List(ctx, filter, page)
}

// Delete enqueues a VM deletion task. The provider deletes the VM from the
// hypervisor; the DB record is removed once the task completes successfully.
func (s *vmService) Delete(ctx context.Context, vmID string) error {
	vm, err := s.vms.GetByID(ctx, vmID)
	if err != nil {
		return fmt.Errorf("vm not found: %w", err)
	}

	h, err := s.hypervisors.GetByID(ctx, vm.HypervisorID.String())
	if err != nil {
		return fmt.Errorf("hypervisor not found: %w", err)
	}

	p, err := s.registry.Get(h.Provider)
	if err != nil {
		return err
	}

	// Mark as deleting so the UI can reflect the transition immediately.
	if err := s.vms.UpdateStatus(ctx, vmID, model.VMStatusDeleting); err != nil {
		s.log.Warn("failed to mark vm as deleting", logger.String("vm_id", vmID), logger.Error(err))
	}

	if err := p.DeleteVM(ctx, vm.ProviderVMID); err != nil {
		// Revert status on failure so the VM is not stuck in deleting.
		_ = s.vms.UpdateStatus(ctx, vmID, model.VMStatusError)
		return fmt.Errorf("provider DeleteVM: %w", err)
	}

	if err := s.vms.Delete(ctx, vmID); err != nil {
		return fmt.Errorf("removing vm record: %w", err)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:     model.AuditActionDelete,
		Resource:   "vm",
		ResourceID: vmID,
		Success:    true,
	})
	return nil
}

func (s *vmService) PowerOn(ctx context.Context, vmID string) (string, error) {
	return s.enqueueVMTask(ctx, vmID, model.TaskTypeVMPowerOn, nil)
}

func (s *vmService) PowerOff(ctx context.Context, vmID string) (string, error) {
	return s.enqueueVMTask(ctx, vmID, model.TaskTypeVMPowerOff, nil)
}

func (s *vmService) Reboot(ctx context.Context, vmID string) (string, error) {
	return s.enqueueVMTask(ctx, vmID, model.TaskTypeVMReboot, nil)
}

func (s *vmService) Suspend(ctx context.Context, vmID string) (string, error) {
	return s.enqueueVMTask(ctx, vmID, model.TaskTypeVMSuspend, nil)
}

func (s *vmService) ListSnapshots(ctx context.Context, vmID string) ([]model.Snapshot, error) {
	// Verify VM exists first.
	if _, err := s.vms.GetByID(ctx, vmID); err != nil {
		return nil, fmt.Errorf("vm not found: %w", err)
	}
	return s.snapshots.ListByVMID(ctx, vmID)
}

func (s *vmService) CreateSnapshot(ctx context.Context, vmID string, spec port.SnapshotSpec) (string, error) {
	return s.enqueueVMTask(ctx, vmID, model.TaskTypeVMSnapshot, model.JSONMap{
		"name":        spec.Name,
		"description": spec.Description,
		"memory":      spec.Memory,
		"quiesce":     spec.Quiesce,
	})
}

func (s *vmService) RevertSnapshot(ctx context.Context, vmID, snapshotID string) (string, error) {
	return s.enqueueVMTask(ctx, vmID, model.TaskTypeVMRestore, model.JSONMap{"snapshot_id": snapshotID})
}

func (s *vmService) DeleteSnapshot(ctx context.Context, vmID, snapshotID string) (string, error) {
	return s.enqueueVMTask(ctx, vmID, model.TaskTypeVMSnapshotDelete, model.JSONMap{
		"snapshot_id": snapshotID,
	})
}

func (s *vmService) GetMetrics(ctx context.Context, vmID string) (*port.VMMetrics, error) {
	vm, err := s.vms.GetByID(ctx, vmID)
	if err != nil {
		return nil, err
	}

	h, err := s.hypervisors.GetByID(ctx, vm.HypervisorID.String())
	if err != nil {
		return nil, err
	}

	p, err := s.registry.Get(h.Provider)
	if err != nil {
		return nil, err
	}

	return p.GetVMMetrics(ctx, vm.ProviderVMID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Bulk operations
// ─────────────────────────────────────────────────────────────────────────────

// BulkPowerOn creates a parent task and fans out individual power-on child tasks.
func (s *vmService) BulkPowerOn(ctx context.Context, vmIDs []string) (string, error) {
	return s.enqueueBulkTask(ctx, vmIDs, model.TaskTypeVMBulkPowerOn, model.TaskTypeVMPowerOn, nil)
}

// BulkPowerOff creates a parent task and fans out individual power-off child tasks.
func (s *vmService) BulkPowerOff(ctx context.Context, vmIDs []string) (string, error) {
	return s.enqueueBulkTask(ctx, vmIDs, model.TaskTypeVMBulkPowerOff, model.TaskTypeVMPowerOff, nil)
}

// BulkReboot creates a parent task and fans out individual reboot child tasks.
func (s *vmService) BulkReboot(ctx context.Context, vmIDs []string) (string, error) {
	return s.enqueueBulkTask(ctx, vmIDs, model.TaskTypeVMBulkReboot, model.TaskTypeVMReboot, nil)
}

// BulkSnapshot creates a parent task and fans out individual snapshot child tasks.
func (s *vmService) BulkSnapshot(ctx context.Context, vmIDs []string, spec port.SnapshotSpec) (string, error) {
	extra := model.JSONMap{
		"name":        spec.Name,
		"description": spec.Description,
		"memory":      spec.Memory,
		"quiesce":     spec.Quiesce,
	}
	return s.enqueueBulkTask(ctx, vmIDs, model.TaskTypeVMBulkSnapshot, model.TaskTypeVMSnapshot, extra)
}

// enqueueBulkTask creates a parent task record and one child task per VM.
// The parent task ID is returned to the caller (HTTP 202). The task engine
// processes each child task independently; the parent is updated by the
// bulk handler once all children complete.
func (s *vmService) enqueueBulkTask(
	ctx context.Context,
	vmIDs []string,
	parentType model.TaskType,
	childType model.TaskType,
	extra model.JSONMap,
) (string, error) {
	if len(vmIDs) == 0 {
		return "", fmt.Errorf("no VM IDs provided")
	}
	if len(vmIDs) > 100 {
		return "", fmt.Errorf("bulk operations are limited to 100 VMs at a time")
	}

	// Load all VMs in one query.
	vms, err := s.vms.GetByIDs(ctx, vmIDs)
	if err != nil {
		return "", fmt.Errorf("loading VMs: %w", err)
	}
	if len(vms) == 0 {
		return "", fmt.Errorf("no valid VMs found for the provided IDs")
	}

	now := time.Now().UTC()
	parentID := uuid.New()

	// Build the parent task.
	parent := &model.Task{
		Base:        model.Base{ID: parentID},
		Type:        parentType,
		Status:      model.TaskStatusPending,
		Priority:    5,
		MaxRetries:  0, // parent is not retried — children handle retries
		ScheduledAt: &now,
		CreatedBy:   callerUUID(ctx),
		Payload: model.JSONMap{
			"vm_ids":     vmIDs,
			"vm_count":   len(vms),
			"child_type": string(childType),
		},
	}
	if err := s.tasks.Create(ctx, parent); err != nil {
		return "", fmt.Errorf("creating parent task: %w", err)
	}

	// Create one child task per VM.
	for i := range vms {
		vm := &vms[i]
		vmUUID := vm.ID
		hypervisorUUID := vm.HypervisorID

		payload := model.JSONMap{
			"vm_id":          vm.ID.String(),
			"provider_vm_id": vm.ProviderVMID,
			"hypervisor_id":  vm.HypervisorID.String(),
		}
		for k, v := range extra {
			payload[k] = v
		}

		child := &model.Task{
			Base:         model.Base{ID: uuid.New()},
			Type:         childType,
			Status:       model.TaskStatusPending,
			Priority:     5,
			MaxRetries:   3,
			VMID:         &vmUUID,
			HypervisorID: &hypervisorUUID,
			ParentTaskID: &parentID,
			ScheduledAt:  &now,
			CreatedBy:    callerUUID(ctx),
			Payload:      payload,
		}
		if err := s.tasks.Create(ctx, child); err != nil {
			s.log.Warn("failed to create child task for bulk operation",
				logger.String("vm_id", vm.ID.String()),
				logger.Error(err),
			)
			// Non-fatal: continue creating tasks for other VMs.
		}
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "vm",
		Description: fmt.Sprintf("bulk %s enqueued for %d VMs", childType, len(vms)),
		Success:     true,
	})

	return parentID.String(), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// enqueueVMTask creates a task record and returns its ID.
func (s *vmService) enqueueVMTask(ctx context.Context, vmID string, taskType model.TaskType, extra model.JSONMap) (string, error) {
	vm, err := s.vms.GetByID(ctx, vmID)
	if err != nil {
		return "", fmt.Errorf("vm not found: %w", err)
	}

	payload := model.JSONMap{
		"vm_id":          vmID,
		"provider_vm_id": vm.ProviderVMID,
		"hypervisor_id":  vm.HypervisorID.String(), // always store as string for consistent deserialization
	}
	for k, v := range extra {
		payload[k] = v
	}

	now := time.Now().UTC()
	vmUUID, _ := uuid.Parse(vmID)
	hypervisorUUID, _ := uuid.Parse(vm.HypervisorID.String())
	t := &model.Task{
		Type:         taskType,
		Status:       model.TaskStatusPending,
		Priority:     5,
		MaxRetries:   3,
		VMID:         &vmUUID,
		HypervisorID: &hypervisorUUID,
		ScheduledAt:  &now,
		Payload:      payload,
		CreatedBy:    callerUUID(ctx),
	}
	t.ID = uuid.New()

	if err := s.tasks.Create(ctx, t); err != nil {
		return "", fmt.Errorf("creating task: %w", err)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "vm",
		ResourceID:  vmID,
		Description: fmt.Sprintf("task %s enqueued", taskType),
		Success:     true,
	})

	return t.ID.String(), nil
}
