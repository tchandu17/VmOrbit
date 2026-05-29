package proxmox

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Proxmox UPID task poller
// ─────────────────────────────────────────────────────────────────────────────

// taskPollConfig controls how long and how often we poll a Proxmox UPID task.
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

// waitForTask polls the Proxmox task identified by upid until it reaches a
// terminal state ("stopped") or the context / timeout is exceeded.
//
// Proxmox UPIDs encode the node name, e.g.:
//
//	UPID:pve:00001234:00000001:5F1A2B3C:qmstart:100:root@pam:
//
// The node is extracted from the UPID so the caller does not need to pass it
// separately. If extraction fails, node is used as a fallback.
func waitForTask(ctx context.Context, client *Client, upid, fallbackNode string, cfg taskPollConfig) error {
	if cfg.interval == 0 {
		cfg.interval = 2 * time.Second
	}

	node := extractNodeFromUPID(upid)
	if node == "" {
		node = fallbackNode
	}
	if node == "" {
		return fmt.Errorf("proxmox: cannot determine node for UPID %q", upid)
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
			return fmt.Errorf("proxmox: task %q timed out: %w", upid, ctx.Err())
		case <-ticker.C:
			status, err := client.GetTaskStatus(ctx, node, upid)
			if err != nil {
				// Transient fetch error — keep polling.
				continue
			}

			if status.Status != "stopped" {
				// Still running or queued.
				continue
			}

			// Task has stopped. Check exit status.
			if status.ExitStatus == "OK" || status.ExitStatus == "" {
				return nil
			}
			return fmt.Errorf("proxmox: task %q failed: %s", upid, status.ExitStatus)
		}
	}
}

// extractNodeFromUPID parses the node name out of a Proxmox UPID string.
// UPID format: UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:
func extractNodeFromUPID(upid string) string {
	// UPIDs always start with "UPID:"
	if !strings.HasPrefix(upid, "UPID:") {
		return ""
	}
	parts := strings.SplitN(upid, ":", 9)
	// parts[0]="UPID", parts[1]=node
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
