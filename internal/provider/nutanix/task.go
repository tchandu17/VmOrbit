package nutanix

import (
	"context"
	"fmt"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Nutanix task poller
// ─────────────────────────────────────────────────────────────────────────────

// taskPollConfig controls how long and how often we poll a Nutanix task UUID.
type taskPollConfig struct {
	// interval between status polls.
	interval time.Duration
	// timeout is the maximum total wait time. Zero means no timeout (rely on ctx).
	timeout time.Duration
}

// defaultTaskPollConfig returns sensible defaults for interactive operations.
func defaultTaskPollConfig() taskPollConfig {
	return taskPollConfig{
		interval: 2 * time.Second,
		timeout:  5 * time.Minute,
	}
}

// waitForTask polls the Nutanix task identified by taskUUID until it reaches a
// terminal state ("SUCCEEDED" or "FAILED") or the context / timeout is exceeded.
//
// Nutanix task states: QUEUED → RUNNING → SUCCEEDED | FAILED | ABORTED
func waitForTask(ctx context.Context, client *Client, taskUUID string, cfg taskPollConfig) error {
	if cfg.interval == 0 {
		cfg.interval = 2 * time.Second
	}

	var cancel context.CancelFunc
	if cfg.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("nutanix: task %q timed out: %w", taskUUID, ctx.Err())
		case <-ticker.C:
			task, err := client.GetTask(ctx, taskUUID)
			if err != nil {
				// Transient fetch error — keep polling.
				continue
			}

			switch task.Status {
			case "SUCCEEDED":
				return nil
			case "FAILED", "ABORTED":
				msg := task.ErrorDetail
				if msg == "" {
					msg = task.ErrorCode
				}
				if msg == "" {
					msg = task.ProgressMessage
				}
				if msg == "" {
					msg = fmt.Sprintf("task %s", task.Status)
				}
				return fmt.Errorf("nutanix: task %q %s: %s", taskUUID, task.Status, msg)
			default:
				// QUEUED or RUNNING — keep polling.
				continue
			}
		}
	}
}

// waitForTaskWithResult polls until completion and returns the first entity
// reference UUID from the task result (useful for create operations).
func waitForTaskWithResult(ctx context.Context, client *Client, taskUUID string, cfg taskPollConfig) (string, error) {
	if cfg.interval == 0 {
		cfg.interval = 2 * time.Second
	}

	var cancel context.CancelFunc
	if cfg.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("nutanix: task %q timed out: %w", taskUUID, ctx.Err())
		case <-ticker.C:
			task, err := client.GetTask(ctx, taskUUID)
			if err != nil {
				continue
			}

			switch task.Status {
			case "SUCCEEDED":
				// Return the first entity UUID from the task result.
				if len(task.EntityReferenceList) > 0 {
					return task.EntityReferenceList[0].UUID, nil
				}
				return "", nil
			case "FAILED", "ABORTED":
				msg := task.ErrorDetail
				if msg == "" {
					msg = task.ErrorCode
				}
				if msg == "" {
					msg = fmt.Sprintf("task %s", task.Status)
				}
				return "", fmt.Errorf("nutanix: task %q %s: %s", taskUUID, task.Status, msg)
			default:
				continue
			}
		}
	}
}
