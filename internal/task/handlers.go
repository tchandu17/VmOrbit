package task

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ─────────────────────────────────────────────────────────────────────────────
// Built-in task handlers
//
// Each handler receives a *TaskContext (which embeds context.Context) and the
// *model.Task being executed. Handlers should:
//   - Call tc.Progress() at meaningful milestones.
//   - Call tc.LogInfo/Warn/Error() for structured per-task logs.
//   - Check tc.IsCancelled() at natural checkpoints and return ErrTaskCancelled.
//
// Power operation handlers (PowerOn/Off/Reboot/Suspend) call the provider
// directly via the registry rather than going through the VM service, which
// would create a new task and cause an infinite loop.
// ─────────────────────────────────────────────────────────────────────────────

// getProviderWithCreds loads the hypervisor credentials from the DB and
// returns the registered provider + decrypted credentials ready for Connect.
// It does NOT call Connect — the caller must do that and defer Disconnect.
func (e *Engine) getProviderWithCreds(tc *TaskContext, hypervisorID string) (port.Provider, port.Credentials, error) {
	creds, providerType, err := e.deps.Services.Hypervisors.BuildCredentials(tc, hypervisorID)
	if err != nil {
		return nil, port.Credentials{}, fmt.Errorf("building credentials for hypervisor %s: %w", hypervisorID, err)
	}

	p, err := e.deps.Registry.Get(providerType)
	if err != nil {
		return nil, port.Credentials{}, fmt.Errorf("provider not registered for type %q: %w", providerType, err)
	}

	return p, creds, nil
}

func (e *Engine) handleVMPowerOn(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	if vmID == "" || providerVMID == "" || hypervisorID == "" {
		return fmt.Errorf("missing vm_id, provider_vm_id, or hypervisor_id in payload")
	}
	tc.LogInfo("powering on VM", map[string]interface{}{
		"vm_id":          vmID,
		"provider_vm_id": providerVMID,
		"hypervisor_id":  hypervisorID,
	})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		tc.LogError("failed to get provider", map[string]interface{}{"error": err.Error()})
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		tc.LogError("provider connect failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "sending power-on command to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	if err := p.PowerOn(tc, providerVMID); err != nil {
		tc.LogError("power-on failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.Progress(80, "updating VM status in database")
	if err := e.deps.VMRepo.UpdateStatus(tc, vmID, model.VMStatusRunning); err != nil {
		tc.LogWarn("failed to update VM status after power-on", map[string]interface{}{"error": err.Error()})
	}

	tc.Progress(100, "VM powered on")
	tc.LogInfo("VM powered on successfully", map[string]interface{}{"vm_id": vmID})
	return nil
}

func (e *Engine) handleVMPowerOff(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	if vmID == "" || providerVMID == "" || hypervisorID == "" {
		return fmt.Errorf("missing vm_id, provider_vm_id, or hypervisor_id in payload")
	}
	tc.LogInfo("powering off VM", map[string]interface{}{
		"vm_id":          vmID,
		"provider_vm_id": providerVMID,
		"hypervisor_id":  hypervisorID,
	})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		tc.LogError("failed to get provider", map[string]interface{}{"error": err.Error()})
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		tc.LogError("provider connect failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "sending power-off command to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	if err := p.PowerOff(tc, providerVMID); err != nil {
		tc.LogError("power-off failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.Progress(80, "updating VM status in database")
	if err := e.deps.VMRepo.UpdateStatus(tc, vmID, model.VMStatusStopped); err != nil {
		tc.LogWarn("failed to update VM status after power-off", map[string]interface{}{"error": err.Error()})
	}

	tc.Progress(100, "VM powered off")
	tc.LogInfo("VM powered off successfully", map[string]interface{}{"vm_id": vmID})
	return nil
}

func (e *Engine) handleVMReboot(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	if vmID == "" || providerVMID == "" || hypervisorID == "" {
		return fmt.Errorf("missing vm_id, provider_vm_id, or hypervisor_id in payload")
	}
	tc.LogInfo("rebooting VM", map[string]interface{}{
		"vm_id":          vmID,
		"provider_vm_id": providerVMID,
		"hypervisor_id":  hypervisorID,
	})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		tc.LogError("failed to get provider", map[string]interface{}{"error": err.Error()})
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		tc.LogError("provider connect failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "sending reboot command to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	if err := p.Reboot(tc, providerVMID); err != nil {
		tc.LogError("reboot failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	// After reboot the VM remains running — no DB status update needed.
	tc.Progress(100, "VM rebooted")
	tc.LogInfo("VM rebooted successfully", map[string]interface{}{"vm_id": vmID})
	return nil
}

func (e *Engine) handleVMSuspend(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	if vmID == "" || providerVMID == "" || hypervisorID == "" {
		return fmt.Errorf("missing vm_id, provider_vm_id, or hypervisor_id in payload")
	}
	tc.LogInfo("suspending VM", map[string]interface{}{
		"vm_id":          vmID,
		"provider_vm_id": providerVMID,
		"hypervisor_id":  hypervisorID,
	})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		tc.LogError("failed to get provider", map[string]interface{}{"error": err.Error()})
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		tc.LogError("provider connect failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "sending suspend command to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	if err := p.Suspend(tc, providerVMID); err != nil {
		tc.LogError("suspend failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.Progress(80, "updating VM status in database")
	if err := e.deps.VMRepo.UpdateStatus(tc, vmID, model.VMStatusSuspended); err != nil {
		tc.LogWarn("failed to update VM status after suspend", map[string]interface{}{"error": err.Error()})
	}

	tc.Progress(100, "VM suspended")
	tc.LogInfo("VM suspended successfully", map[string]interface{}{"vm_id": vmID})
	return nil
}

func (e *Engine) handleVMDelete(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	if vmID == "" {
		return fmt.Errorf("missing vm_id in payload")
	}
	tc.LogInfo("deleting VM", map[string]interface{}{"vm_id": vmID})
	tc.Progress(10, "initiating VM deletion")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	if err := e.deps.Services.VMs.Delete(tc, vmID); err != nil {
		tc.LogError("delete failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.Progress(100, "VM deleted")
	tc.LogInfo("VM deleted successfully", map[string]interface{}{"vm_id": vmID})
	return nil
}

func (e *Engine) handleVMSnapshot(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	if vmID == "" || providerVMID == "" || hypervisorID == "" {
		return fmt.Errorf("missing vm_id, provider_vm_id, or hypervisor_id in payload")
	}
	spec := port.SnapshotSpec{
		Name:        stringFromPayload(t.Payload, "name"),
		Description: stringFromPayload(t.Payload, "description"),
		Memory:      boolFromPayload(t.Payload, "memory"),
		Quiesce:     boolFromPayload(t.Payload, "quiesce"),
	}
	tc.LogInfo("creating snapshot", map[string]interface{}{"vm_id": vmID, "name": spec.Name})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		tc.LogError("failed to get provider", map[string]interface{}{"error": err.Error()})
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		tc.LogError("provider connect failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "creating snapshot on hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	info, err := p.CreateSnapshot(tc, providerVMID, spec)
	if err != nil {
		tc.LogError("snapshot failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.Progress(80, "persisting snapshot record")

	// Persist the snapshot to the database.
	if e.deps.SnapshotRepo != nil && info != nil {
		vmUUID, parseErr := uuid.Parse(vmID)
		if parseErr == nil {
			snap := &model.Snapshot{
				VMID:        vmUUID,
				ProviderID:  info.ProviderID,
				Name:        info.Name,
				Description: info.Description,
				IsCurrent:   true,
			}
			if upsertErr := e.deps.SnapshotRepo.BulkUpsert(tc, []model.Snapshot{*snap}); upsertErr != nil {
				tc.LogWarn("failed to persist snapshot record", map[string]interface{}{"error": upsertErr.Error()})
			} else {
				// Mark this as the current snapshot, clear others.
				if saved, getErr := e.deps.SnapshotRepo.GetByProviderID(tc, vmID, info.ProviderID); getErr == nil {
					_ = e.deps.SnapshotRepo.SetCurrentSnapshot(tc, vmID, saved.ID.String())
				}
			}
		}
	}

	tc.Progress(100, "snapshot created")
	tc.LogInfo("snapshot created successfully", map[string]interface{}{"vm_id": vmID, "snapshot": spec.Name})
	return nil
}

func (e *Engine) handleVMSnapshotDelete(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	snapshotID := stringFromPayload(t.Payload, "snapshot_id")
	if vmID == "" || providerVMID == "" || hypervisorID == "" || snapshotID == "" {
		return fmt.Errorf("missing vm_id, provider_vm_id, hypervisor_id, or snapshot_id in payload")
	}
	tc.LogInfo("deleting snapshot", map[string]interface{}{"vm_id": vmID, "snapshot_id": snapshotID})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		tc.LogError("failed to get provider", map[string]interface{}{"error": err.Error()})
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		tc.LogError("provider connect failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "deleting snapshot on hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	// snapshotID may be a DB UUID or a provider-native ID.
	// Try to resolve the provider-native ID from the DB first.
	providerSnapshotID := snapshotID
	if e.deps.SnapshotRepo != nil {
		if snap, dbErr := e.deps.SnapshotRepo.GetByID(tc, snapshotID); dbErr == nil {
			providerSnapshotID = snap.ProviderID
		}
	}

	if err := p.DeleteSnapshot(tc, providerVMID, providerSnapshotID); err != nil {
		tc.LogError("snapshot delete failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.Progress(80, "removing snapshot record from database")

	// Remove the snapshot record from the database.
	if e.deps.SnapshotRepo != nil {
		if delErr := e.deps.SnapshotRepo.Delete(tc, snapshotID); delErr != nil {
			tc.LogWarn("failed to remove snapshot record", map[string]interface{}{"error": delErr.Error()})
		}
	}

	tc.Progress(100, "snapshot deleted")
	tc.LogInfo("snapshot deleted successfully", map[string]interface{}{"vm_id": vmID})
	return nil
}

func (e *Engine) handleVMRestore(tc *TaskContext, t *model.Task) error {
	vmID := stringFromPayload(t.Payload, "vm_id")
	providerVMID := stringFromPayload(t.Payload, "provider_vm_id")
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	snapshotID := stringFromPayload(t.Payload, "snapshot_id")
	if vmID == "" || providerVMID == "" || hypervisorID == "" || snapshotID == "" {
		return fmt.Errorf("missing vm_id, provider_vm_id, hypervisor_id, or snapshot_id in payload")
	}
	tc.LogInfo("restoring snapshot", map[string]interface{}{"vm_id": vmID, "snapshot_id": snapshotID})
	tc.Progress(10, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	p, creds, err := e.getProviderWithCreds(tc, hypervisorID)
	if err != nil {
		tc.LogError("failed to get provider", map[string]interface{}{"error": err.Error()})
		return err
	}
	if err := p.Connect(tc, creds); err != nil {
		tc.LogError("provider connect failed", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(tc) //nolint:errcheck

	tc.Progress(30, "reverting to snapshot on hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	if err := p.RevertSnapshot(tc, providerVMID, snapshotID); err != nil {
		tc.LogError("restore failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	tc.Progress(100, "snapshot restored")
	tc.LogInfo("snapshot restored successfully", map[string]interface{}{"vm_id": vmID})
	return nil
}

func (e *Engine) handleInventorySync(tc *TaskContext, t *model.Task) error {
	hypervisorID := stringFromPayload(t.Payload, "hypervisor_id")
	if hypervisorID == "" {
		return fmt.Errorf("missing hypervisor_id in payload")
	}
	tc.LogInfo("starting inventory sync", map[string]interface{}{"hypervisor_id": hypervisorID})
	tc.Progress(5, "connecting to hypervisor")

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	// progress callback bridges TaskContext progress reporting into SyncInventoryNow.
	progressFn := func(pct int, msg string) {
		if tc.IsCancelled() {
			return
		}
		tc.Progress(pct, msg)
		tc.LogInfo(msg, map[string]interface{}{"hypervisor_id": hypervisorID, "progress": pct})
	}

	result, err := e.deps.Services.Hypervisors.SyncInventoryNow(tc, hypervisorID, progressFn)
	if err != nil {
		tc.LogError("inventory sync failed", map[string]interface{}{"error": err.Error()})
		return err
	}

	if len(result.Errors) > 0 {
		for _, syncErr := range result.Errors {
			tc.LogWarn("partial sync error", map[string]interface{}{"error": syncErr})
		}
	}

	tc.LogInfo("inventory sync completed", map[string]interface{}{
		"hypervisor_id":     hypervisorID,
		"vms_updated":       result.VMsUpdated,
		"vms_removed":       result.VMsRemoved,
		"stores_upserted":   result.StoresUpserted,
		"networks_upserted": result.NetworksUpserted,
	})
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Payload extraction helpers
// ─────────────────────────────────────────────────────────────────────────────

func stringFromPayload(p model.JSONMap, key string) string {
	if p == nil {
		return ""
	}
	// The payload value may be stored as a string or as a fmt.Stringer (e.g. uuid.UUID).
	switch v := p[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		if v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
}

func boolFromPayload(p model.JSONMap, key string) bool {
	if p == nil {
		return false
	}
	v, _ := p[key].(bool)
	return v
}
