package task

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	pkglogger "github.com/vmOrbit/backend/pkg/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// Environment orchestration handlers
//
// Each handler:
//  1. Loads the OrchestrationRun from the DB (via run_id in payload).
//  2. Iterates through steps in execution order.
//  3. For each step, dispatches the appropriate VM task and waits for completion.
//  4. Updates step status and run progress after each VM.
//  5. On partial failure: marks failed steps, continues (partial failure handling).
//  6. Updates the final run status.
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) handleEnvStart(tc *TaskContext, t *model.Task) error {
	return e.handleEnvOperation(tc, t, model.OrchestrationOpStart)
}

func (e *Engine) handleEnvStop(tc *TaskContext, t *model.Task) error {
	return e.handleEnvOperation(tc, t, model.OrchestrationOpStop)
}

func (e *Engine) handleEnvRestart(tc *TaskContext, t *model.Task) error {
	return e.handleEnvOperation(tc, t, model.OrchestrationOpRestart)
}

func (e *Engine) handleEnvSnapshot(tc *TaskContext, t *model.Task) error {
	return e.handleEnvOperation(tc, t, model.OrchestrationOpSnapshot)
}

func (e *Engine) handleEnvClone(tc *TaskContext, t *model.Task) error {
	return e.handleEnvOperation(tc, t, model.OrchestrationOpClone)
}

// handleEnvOperation is the shared orchestration executor.
func (e *Engine) handleEnvOperation(tc *TaskContext, t *model.Task, op model.OrchestrationOperation) error {
	runID := stringFromPayload(t.Payload, "run_id")
	environmentID := stringFromPayload(t.Payload, "environment_id")
	if runID == "" || environmentID == "" {
		return fmt.Errorf("missing run_id or environment_id in payload")
	}

	tc.LogInfo("starting environment orchestration", map[string]interface{}{
		"environment_id": environmentID,
		"run_id":         runID,
		"operation":      string(op),
	})
	tc.Progress(5, "loading orchestration plan")

	// Mark run as running
	if err := e.deps.Services.Environments.UpdateRunStatus(tc, runID, model.OrchestrationRunStatusRunning, ""); err != nil {
		tc.LogWarn("failed to mark run as running", map[string]interface{}{"error": err.Error()})
	}

	// Load steps
	steps, err := e.deps.Services.Environments.GetRunSteps(tc, runID)
	if err != nil {
		_ = e.deps.Services.Environments.UpdateRunStatus(tc, runID, model.OrchestrationRunStatusFailed, err.Error())
		return fmt.Errorf("loading orchestration steps: %w", err)
	}

	if len(steps) == 0 {
		_ = e.deps.Services.Environments.UpdateRunStatus(tc, runID, model.OrchestrationRunStatusCompleted, "")
		tc.Progress(100, "no VMs to orchestrate")
		return nil
	}

	tc.Progress(10, fmt.Sprintf("executing %d VM steps", len(steps)))

	completed, failed, skipped := 0, 0, 0
	total := len(steps)

	for i, step := range steps {
		if tc.IsCancelled() {
			_ = e.deps.Services.Environments.UpdateRunStatus(tc, runID, model.OrchestrationRunStatusCancelled, "cancelled by user")
			return ErrTaskCancelled
		}

		vmID := step.VMID.String()
		stepID := step.ID.String()

		tc.LogInfo("executing step", map[string]interface{}{
			"step":           i + 1,
			"total":          total,
			"vm_id":          vmID,
			"execution_order": step.ExecutionOrder,
		})

		// Mark step as running
		_ = e.deps.Services.Environments.UpdateStepStatus(tc, stepID, model.OrchestrationStepStatusRunning, nil, "")

		// Dispatch the VM-level task
		taskID, stepErr := e.dispatchVMTask(tc, vmID, op, t.Payload)
		if stepErr != nil {
			tc.LogError("failed to dispatch vm task", map[string]interface{}{
				"vm_id": vmID,
				"error": stepErr.Error(),
			})
			_ = e.deps.Services.Environments.UpdateStepStatus(tc, stepID, model.OrchestrationStepStatusFailed, nil, stepErr.Error())
			failed++
		} else {
			// Wait for the dispatched task to complete
			waitErr := e.waitForTask(tc, taskID)
			if waitErr != nil {
				tc.LogError("vm task failed", map[string]interface{}{
					"vm_id":   vmID,
					"task_id": taskID,
					"error":   waitErr.Error(),
				})
				_ = e.deps.Services.Environments.UpdateStepStatus(tc, stepID, model.OrchestrationStepStatusFailed, &taskID, waitErr.Error())
				failed++
			} else {
				_ = e.deps.Services.Environments.UpdateStepStatus(tc, stepID, model.OrchestrationStepStatusCompleted, &taskID, "")
				completed++
			}
		}

		// Update run progress
		progress := 10 + int(float64(i+1)/float64(total)*85)
		_ = e.deps.Services.Environments.UpdateRunProgress(tc, runID, progress, completed, failed, skipped)

		tc.Progress(progress, fmt.Sprintf("step %d/%d: vm %s", i+1, total, vmID))
	}

	// Determine final status
	finalStatus := model.OrchestrationRunStatusCompleted
	if failed > 0 && completed == 0 {
		finalStatus = model.OrchestrationRunStatusFailed
	} else if failed > 0 {
		// Partial success — still mark completed but log failures
		finalStatus = model.OrchestrationRunStatusCompleted
	}

	errMsg := ""
	if failed > 0 {
		errMsg = fmt.Sprintf("%d of %d VMs failed", failed, total)
	}

	_ = e.deps.Services.Environments.UpdateRunStatus(tc, runID, finalStatus, errMsg)
	_ = e.deps.Services.Environments.UpdateRunProgress(tc, runID, 100, completed, failed, skipped)

	// Refresh environment aggregate status
	go func() {
		_, _ = e.deps.Services.Environments.RefreshStatus(tc, environmentID)
	}()

	tc.Progress(100, fmt.Sprintf("orchestration complete: %d completed, %d failed", completed, failed))
	tc.LogInfo("environment orchestration complete", map[string]interface{}{
		"environment_id": environmentID,
		"run_id":         runID,
		"completed":      completed,
		"failed":         failed,
		"skipped":        skipped,
	})

	if failed > 0 && completed == 0 {
		return fmt.Errorf("all %d VMs failed during orchestration", failed)
	}

	return nil
}

// dispatchVMTask creates and enqueues a VM-level task for the given operation.
// Returns the task ID string.
func (e *Engine) dispatchVMTask(tc *TaskContext, vmID string, op model.OrchestrationOperation, parentPayload model.JSONMap) (string, error) {
	vm, err := e.deps.VMRepo.GetByID(tc, vmID)
	if err != nil {
		return "", fmt.Errorf("vm not found: %w", err)
	}

	var taskType model.TaskType
	var payload model.JSONMap

	basePayload := model.JSONMap{
		"vm_id":          vm.ID.String(),
		"provider_vm_id": vm.ProviderVMID,
		"hypervisor_id":  vm.HypervisorID.String(),
	}

	switch op {
	case model.OrchestrationOpStart:
		taskType = model.TaskTypeVMPowerOn
		payload = basePayload

	case model.OrchestrationOpStop:
		taskType = model.TaskTypeVMPowerOff
		payload = basePayload

	case model.OrchestrationOpRestart:
		taskType = model.TaskTypeVMReboot
		payload = basePayload

	case model.OrchestrationOpSnapshot:
		taskType = model.TaskTypeVMSnapshot
		snapshotName := stringFromPayload(parentPayload, "snapshot_name")
		if snapshotName == "" {
			snapshotName = fmt.Sprintf("env-snapshot-%s", time.Now().UTC().Format("20060102-150405"))
		}
		payload = model.JSONMap{
			"vm_id":          vm.ID.String(),
			"provider_vm_id": vm.ProviderVMID,
			"hypervisor_id":  vm.HypervisorID.String(),
			"name":           fmt.Sprintf("%s-%s", snapshotName, vm.Name),
			"description":    stringFromPayload(parentPayload, "description"),
			"memory":         boolFromPayload(parentPayload, "memory"),
		}

	case model.OrchestrationOpClone:
		taskType = model.TaskTypeVMCloneOp
		nameSuffix := stringFromPayload(parentPayload, "name_suffix")
		if nameSuffix == "" {
			nameSuffix = "-clone"
		}
		payload = model.JSONMap{
			"vm_id":          vm.ID.String(),
			"provider_vm_id": vm.ProviderVMID,
			"hypervisor_id":  vm.HypervisorID.String(),
			"name":           vm.Name + nameSuffix,
		}

	default:
		return "", fmt.Errorf("unsupported orchestration operation: %s", op)
	}

	// Create the child task
	taskID := uuid.New()
	now := time.Now().UTC()
	vmUUID := vm.ID
	hypervisorUUID := vm.HypervisorID
	childTask := &model.Task{
		Type:         taskType,
		Status:       model.TaskStatusPending,
		Priority:     5,
		MaxRetries:   2,
		VMID:         &vmUUID,
		HypervisorID: &hypervisorUUID,
		ScheduledAt:  &now,
		Payload:      payload,
	}
	childTask.ID = taskID

	if err := e.deps.TaskRepo.Create(tc, childTask); err != nil {
		return "", fmt.Errorf("creating child task: %w", err)
	}

	// Enqueue immediately
	if err := e.Enqueue(tc, taskID.String(), 5); err != nil {
		e.deps.Log.Warn("failed to enqueue child task (poller will recover)",
			pkglogger.String("task_id", taskID.String()),
			pkglogger.Error(err),
		)
	}

	return taskID.String(), nil
}

// waitForTask polls the DB until the task reaches a terminal state.
// It respects the TaskContext cancellation signal.
func (e *Engine) waitForTask(tc *TaskContext, taskID string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.NewTimer(e.deps.Config.DefaultTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-tc.Done():
			return ErrTaskCancelled
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for task %s", taskID)
		case <-ticker.C:
			t, err := e.deps.TaskRepo.GetByID(tc, taskID)
			if err != nil {
				return fmt.Errorf("polling task %s: %w", taskID, err)
			}
			switch t.Status {
			case model.TaskStatusCompleted:
				return nil
			case model.TaskStatusFailed:
				msg := t.ErrorMessage
				if msg == "" {
					msg = "task failed"
				}
				return fmt.Errorf("%s", msg)
			case model.TaskStatusCancelled:
				return fmt.Errorf("task was cancelled")
			case model.TaskStatusTimedOut:
				return fmt.Errorf("task timed out")
			}
			// Still running/pending/queued/retrying — keep polling
		}
	}
}
