package task

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// handleVMBulkPowerOn is the parent task handler for bulk power-on.
// It fans out to child tasks (already created by the service) and waits
// for them to complete, tracking overall progress.
func (e *Engine) handleVMBulkPowerOn(tc *TaskContext, t *model.Task) error {
	return e.handleBulkParent(tc, t, model.TaskTypeVMPowerOn, "power-on")
}

// handleVMBulkPowerOff is the parent task handler for bulk power-off.
func (e *Engine) handleVMBulkPowerOff(tc *TaskContext, t *model.Task) error {
	return e.handleBulkParent(tc, t, model.TaskTypeVMPowerOff, "power-off")
}

// handleVMBulkReboot is the parent task handler for bulk reboot.
func (e *Engine) handleVMBulkReboot(tc *TaskContext, t *model.Task) error {
	return e.handleBulkParent(tc, t, model.TaskTypeVMReboot, "reboot")
}

// handleVMBulkSnapshot is the parent task handler for bulk snapshot.
func (e *Engine) handleVMBulkSnapshot(tc *TaskContext, t *model.Task) error {
	return e.handleBulkParent(tc, t, model.TaskTypeVMSnapshot, "snapshot")
}

// handleBulkParent loads all child tasks for this parent, enqueues them into
// Redis, and polls until they all reach a terminal state. Progress is reported
// as (completed / total) * 100.
//
// Design notes:
//   - Child tasks are already created in the DB by the service layer.
//   - We enqueue them here (inside the worker) so they get immediate dispatch.
//   - We poll the DB for child completion rather than blocking on channels,
//     which keeps the implementation simple and crash-safe.
//   - Partial failures are tolerated: the parent completes with a warning log
//     listing failed child task IDs.
func (e *Engine) handleBulkParent(tc *TaskContext, parent *model.Task, childType model.TaskType, opName string) error {
	parentID := parent.ID.String()

	tc.LogInfo(fmt.Sprintf("starting bulk %s", opName), map[string]interface{}{
		"parent_task_id": parentID,
	})
	tc.Progress(5, fmt.Sprintf("loading child tasks for bulk %s", opName))

	if tc.IsCancelled() {
		return ErrTaskCancelled
	}

	// Load all child tasks for this parent.
	children, err := e.deps.TaskRepo.ListByParentID(tc, parentID)
	if err != nil {
		return fmt.Errorf("loading child tasks: %w", err)
	}
	if len(children) == 0 {
		tc.LogWarn("no child tasks found for bulk operation", map[string]interface{}{
			"parent_task_id": parentID,
		})
		return nil
	}

	total := len(children)
	tc.LogInfo(fmt.Sprintf("enqueueing %d child tasks", total), map[string]interface{}{
		"parent_task_id": parentID,
		"child_type":     string(childType),
	})

	// Enqueue all child tasks into Redis for immediate dispatch.
	for i := range children {
		child := &children[i]
		if err := e.queue.Enqueue(tc, child.ID.String(), child.Priority); err != nil {
			tc.LogWarn("failed to enqueue child task (poller will recover)", map[string]interface{}{
				"child_task_id": child.ID.String(),
				"error":         err.Error(),
			})
		} else {
			_ = e.deps.TaskRepo.UpdateStatus(tc, child.ID.String(), model.TaskStatusQueued, nil, "")
		}
	}

	tc.Progress(10, fmt.Sprintf("dispatched %d tasks, waiting for completion", total))

	// Poll for child task completion.
	var completed int64
	var failed []string
	var mu sync.Mutex

	// Use a wait-group style poll: check all children every 2 seconds.
	childIDs := make([]string, len(children))
	for i, c := range children {
		childIDs[i] = c.ID.String()
	}

	remaining := make(map[string]bool, total)
	for _, id := range childIDs {
		remaining[id] = true
	}

	for len(remaining) > 0 {
		if tc.IsCancelled() {
			return ErrTaskCancelled
		}

		// Check each remaining child.
		for childID := range remaining {
			child, err := e.deps.TaskRepo.GetByID(tc, childID)
			if err != nil {
				continue
			}
			switch child.Status {
			case model.TaskStatusCompleted:
				atomic.AddInt64(&completed, 1)
				delete(remaining, childID)
			case model.TaskStatusFailed, model.TaskStatusCancelled, model.TaskStatusTimedOut:
				atomic.AddInt64(&completed, 1)
				mu.Lock()
				failed = append(failed, childID)
				mu.Unlock()
				delete(remaining, childID)
			}
		}

		// Report progress.
		done := int(atomic.LoadInt64(&completed))
		pct := 10 + int(float64(done)/float64(total)*85)
		tc.Progress(pct, fmt.Sprintf("%d/%d tasks completed", done, total))

		if len(remaining) > 0 {
			// Sleep before next poll cycle to avoid busy-waiting.
			select {
			case <-tc.Done():
				return ErrTaskCancelled
			case <-time.After(2 * time.Second):
			}
		}
	}

	tc.Progress(100, fmt.Sprintf("bulk %s complete: %d/%d succeeded", opName, total-len(failed), total))

	if len(failed) > 0 {
		tc.LogWarn(fmt.Sprintf("bulk %s: %d/%d tasks failed", opName, len(failed), total), map[string]interface{}{
			"failed_task_ids": failed,
		})
		// Return nil — partial failure is not a fatal error for the parent task.
		// The individual child task errors are visible in the task list.
	} else {
		tc.LogInfo(fmt.Sprintf("bulk %s completed successfully", opName), map[string]interface{}{
			"total": total,
		})
	}

	return nil
}
