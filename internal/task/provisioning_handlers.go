package task

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// Template sync handler
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) handleTemplateSync(tc *TaskContext, t *model.Task) error {
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	if hypervisorID == "" {
		return fmt.Errorf("missing hypervisor_id in payload")
	}
	tc.LogInfo("starting template sync", map[string]interface{}{"hypervisor_id": hypervisorID})
	tc.Progress(5, "starting template discovery")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	progressFn := func(pct int, msg string) {
		if tc.IsCancelled() {
			return
		}
		tc.Progress(pct, msg)
		tc.LogInfo(msg, map[string]interface{}{"hypervisor_id": hypervisorID, "progress": pct})
	}

	count, err := e.deps.Services.Templates.SyncTemplatesNow(tc, hypervisorID, progressFn)
	if err != nil {
		tc.LogError("template sync failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.LogInfo("template sync completed", map[string]interface{}{
		"hypervisor_id": hypervisorID,
		"count":         count,
	})
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// VM clone handler
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) handleVMClone(tc *TaskContext, t *model.Task) error {
	jobID := stringFromPayload(t.Payload, "provisioning_job_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	name := stringFromPayload(t.Payload, "name")
	dataStore := stringFromPayload(t.Payload, "data_store")
	linked := boolFromPayload(t.Payload, "linked")

	if jobID == "" || providerVMID == "" || hypervisorID == "" || name == "" {
		return fmt.Errorf("missing required fields in clone payload")
	}

	tc.LogInfo("cloning VM", map[string]interface{}{
		"job_id":         jobID,
		"provider_vm_id": providerVMID,
		"name":           name,
	})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	// Mark job as running.
	e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusRunning, "")

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusFailed, err.Error())
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusFailed, err.Error())
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "cloning VM on hypervisor")

	if tc.IsCancelled() {
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusCancelled, "cancelled")
		return ErrTaskCancelled
	}

	cloneSpec := port.VMCloneSpec{
		Name:      name,
		DataStore: dataStore,
		Linked:    linked,
	}

	newVMInfo, err := p.CloneVM(tc, providerVMID, cloneSpec)
	if err != nil {
		tc.LogError("clone failed", map[string]interface{}{"error": err.Error()})
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusFailed, err.Error())
		// CodeUnsupported means the provider fundamentally cannot clone
		// (e.g. standalone ESXi). Wrap as a permanent failure so the task
		// engine does not retry — retrying would never succeed.
		var pe *provider.ProviderError
		if errors.As(err, &pe) && pe.Code == provider.CodeUnsupported {
			return fmt.Errorf("%w: %s", ErrTaskPermanentFailure, err.Error())
		}
		return err
	}

	tc.Progress(80, "persisting cloned VM to database")

	// The cloned VM will be picked up on the next inventory sync.
	// For immediate visibility, trigger a sync or create a stub record.
	// We update the job with the new provider VM ID for reference.
	if e.deps.ProvisioningRepo != nil {
		job, getErr := e.deps.ProvisioningRepo.GetByID(tc, jobID)
		if getErr == nil {
			job.Status = model.ProvisioningJobStatusCompleted
			job.ErrorMessage = ""
			// Store the new provider VM ID in metadata for reference.
			if job.Metadata == nil {
				job.Metadata = model.JSONMap{}
			}
			job.Metadata["result_provider_vm_id"] = newVMInfo.ProviderVMID
			_ = e.deps.ProvisioningRepo.Update(tc, job)
		}
	}

	tc.Progress(100, "VM cloned successfully")
	tc.LogInfo("VM cloned successfully", map[string]interface{}{
		"job_id":             jobID,
		"new_provider_vm_id": newVMInfo.ProviderVMID,
		"new_vm_name":        newVMInfo.Name,
	})
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// VM provision handler
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) handleVMProvision(tc *TaskContext, t *model.Task) error {
	jobID := stringFromPayload(t.Payload, "provisioning_job_id")
	providerTemplateID := stringFromPayload(t.Payload, "provider_template_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	name := stringFromPayload(t.Payload, "name")
	dataStore := stringFromPayload(t.Payload, "data_store")
	networkName := stringFromPayload(t.Payload, "network_name")
	cpuCount := intFromPayload(t.Payload, "cpu_count")
	memoryMB := intFromPayload(t.Payload, "memory_mb")
	diskGB := intFromPayload(t.Payload, "disk_gb")

	if jobID == "" || providerTemplateID == "" || hypervisorID == "" || name == "" {
		return fmt.Errorf("missing required fields in provision payload")
	}

	tc.LogInfo("provisioning VM from template", map[string]interface{}{
		"job_id":               jobID,
		"provider_template_id": providerTemplateID,
		"name":                 name,
	})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	// Mark job as running.
	e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusRunning, "")

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusFailed, err.Error())
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusFailed, err.Error())
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "provisioning VM from template on hypervisor")

	if tc.IsCancelled() {
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusCancelled, "cancelled")
		return ErrTaskCancelled
	}

	createSpec := port.VMCreateSpec{
		Name:        name,
		CPUCount:    cpuCount,
		MemoryMB:    memoryMB,
		DiskGB:      diskGB,
		NetworkName: networkName,
		DataStore:   dataStore,
		TemplateID:  providerTemplateID,
	}

	newVMInfo, err := p.CreateVM(tc, createSpec)
	if err != nil {
		tc.LogError("provision failed", map[string]interface{}{"error": err.Error()})
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusFailed, err.Error())
		return err
	}

	tc.Progress(80, "persisting provisioned VM to database")

	// Persist the new VM record immediately so it appears in the inventory.
	if e.deps.VMRepo != nil && e.deps.HypervisorRepo != nil {
		hypervisorUUID, parseErr := uuid.Parse(hypervisorID)
		if parseErr == nil {
			newVM := model.VM{
				HypervisorID: hypervisorUUID,
				ProviderVMID: newVMInfo.ProviderVMID,
				Name:         newVMInfo.Name,
				Status:       model.VMStatusStopped,
				CPUCount:     newVMInfo.CPUCount,
				MemoryMB:     newVMInfo.MemoryMB,
				DiskGB:       newVMInfo.DiskGB,
				GuestOS:      newVMInfo.GuestOS,
				GuestOSType:  newVMInfo.GuestOSType,
			}
			newVM.ID = uuid.New()
			if len(newVMInfo.Extra) > 0 {
				newVM.Metadata = model.JSONMap{}
				for k, v := range newVMInfo.Extra {
					newVM.Metadata[k] = v
				}
			}
			if upsertErr := e.deps.VMRepo.BulkUpsert(tc, []model.VM{newVM}); upsertErr != nil {
				tc.LogWarn("failed to persist new VM record", map[string]interface{}{"error": upsertErr.Error()})
			} else if e.deps.ProvisioningRepo != nil {
				// Link the result VM to the job.
				if job, getErr := e.deps.ProvisioningRepo.GetByID(tc, jobID); getErr == nil {
					// Find the persisted VM by provider ID.
					if persistedVM, vmErr := e.deps.VMRepo.GetByProviderID(tc, hypervisorID, newVMInfo.ProviderVMID); vmErr == nil {
						resultVMID := persistedVM.ID
						job.ResultVMID = &resultVMID
					}
					job.Status = model.ProvisioningJobStatusCompleted
					job.ErrorMessage = ""
					_ = e.deps.ProvisioningRepo.Update(tc, job)
				}
			}
		}
	} else {
		// Just mark the job complete without a VM link.
		e.updateJobStatus(tc, jobID, model.ProvisioningJobStatusCompleted, "")
	}

	tc.Progress(100, "VM provisioned successfully")
	tc.LogInfo("VM provisioned successfully", map[string]interface{}{
		"job_id":             jobID,
		"new_provider_vm_id": newVMInfo.ProviderVMID,
		"new_vm_name":        newVMInfo.Name,
	})
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// updateJobStatus updates the provisioning job status in the database.
// Non-fatal — failures are logged but do not affect the task outcome.
func (e *Engine) updateJobStatus(tc *TaskContext, jobID string, status model.ProvisioningJobStatus, errMsg string) {
	if e.deps.ProvisioningRepo == nil {
		return
	}
	job, err := e.deps.ProvisioningRepo.GetByID(tc, jobID)
	if err != nil {
		return
	}
	job.Status = status
	job.ErrorMessage = errMsg
	if updateErr := e.deps.ProvisioningRepo.Update(tc, job); updateErr != nil {
		e.deps.Log.Warn("failed to update provisioning job status",
			logger.String("job_id", jobID),
			logger.Error(updateErr),
		)
	}
}

// intFromPayload extracts an int from a task payload, handling float64 (JSON default).
func intFromPayload(p model.JSONMap, key string) int {
	if p == nil {
		return 0
	}
	switch v := p[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
