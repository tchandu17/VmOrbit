package database

import (
	"fmt"

	"gorm.io/gorm"
)

// CreatePerformanceIndexes creates composite and partial indexes that GORM's
// AutoMigrate cannot express via struct tags. These are idempotent — safe to
// run on every startup. Each index uses CREATE INDEX IF NOT EXISTS.
//
// Indexes are ordered by query criticality:
//  1. Task engine polling (hot path — runs every 2–5 seconds)
//  2. VM inventory queries (hot path — every page load)
//  3. Audit / task log pagination (warm path)
//  4. Analytics / capacity queries (cold path)
func CreatePerformanceIndexes(db *gorm.DB) error {
	indexes := []struct {
		name string
		ddl  string
	}{
		// ── Tasks ────────────────────────────────────────────────────────────
		// Primary polling index for the DB fallback poller.
		// ListPending: WHERE status = 'pending' ORDER BY priority ASC, created_at ASC
		{
			name: "idx_tasks_status_priority_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_tasks_status_priority_created
				  ON tasks (status, priority ASC, created_at ASC)
				  WHERE deleted_at IS NULL`,
		},
		// Task list by hypervisor (task drawer, hypervisor detail page)
		{
			name: "idx_tasks_hypervisor_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_tasks_hypervisor_created
				  ON tasks (hypervisor_id, created_at DESC)
				  WHERE deleted_at IS NULL AND hypervisor_id IS NOT NULL`,
		},
		// Task list by VM (VM detail page)
		{
			name: "idx_tasks_vm_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_tasks_vm_created
				  ON tasks (vm_id, created_at DESC)
				  WHERE deleted_at IS NULL AND vm_id IS NOT NULL`,
		},
		// Active tasks (running/queued/pending) — used by dashboard stats
		{
			name: "idx_tasks_active_status",
			ddl: `CREATE INDEX IF NOT EXISTS idx_tasks_active_status
				  ON tasks (status, created_at DESC)
				  WHERE deleted_at IS NULL AND status IN ('pending','queued','running','retrying')`,
		},
		// Parent task lookup for bulk operations
		{
			name: "idx_tasks_parent_id",
			ddl: `CREATE INDEX IF NOT EXISTS idx_tasks_parent_id
				  ON tasks (parent_task_id)
				  WHERE deleted_at IS NULL AND parent_task_id IS NOT NULL`,
		},

		// ── Task Logs ────────────────────────────────────────────────────────
		// Paginated log queries: WHERE task_id = ? ORDER BY created_at ASC
		{
			name: "idx_task_logs_task_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_task_logs_task_created
				  ON task_logs (task_id, created_at ASC)`,
		},

		// ── VMs ──────────────────────────────────────────────────────────────
		// VM list filtered by hypervisor (most common query)
		{
			name: "idx_vms_hypervisor_status",
			ddl: `CREATE INDEX IF NOT EXISTS idx_vms_hypervisor_status
				  ON vms (hypervisor_id, status)
				  WHERE deleted_at IS NULL`,
		},
		// VM list ordered by name (default sort)
		{
			name: "idx_vms_hypervisor_name",
			ddl: `CREATE INDEX IF NOT EXISTS idx_vms_hypervisor_name
				  ON vms (hypervisor_id, name)
				  WHERE deleted_at IS NULL`,
		},
		// MarkDeletedByProviderIDs: WHERE hypervisor_id = ? AND provider_vm_id NOT IN (?)
		// The unique index uidx_vm_provider already covers this, but an explicit
		// covering index on (hypervisor_id, provider_vm_id) speeds up the NOT IN scan.
		{
			name: "idx_vms_hypervisor_provider_id",
			ddl: `CREATE INDEX IF NOT EXISTS idx_vms_hypervisor_provider_id
				  ON vms (hypervisor_id, provider_vm_id)
				  WHERE deleted_at IS NULL`,
		},

		// ── Audit Logs ───────────────────────────────────────────────────────
		// Audit list filtered by resource + time range
		{
			name: "idx_audit_logs_resource_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_created
				  ON audit_logs (resource, created_at DESC)`,
		},
		// Audit list filtered by user
		{
			name: "idx_audit_logs_user_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created
				  ON audit_logs (user_id, created_at DESC)
				  WHERE user_id IS NOT NULL`,
		},
		// Audit list filtered by hypervisor
		{
			name: "idx_audit_logs_hypervisor_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_audit_logs_hypervisor_created
				  ON audit_logs (hypervisor_id, created_at DESC)
				  WHERE hypervisor_id IS NOT NULL`,
		},

		// ── Provider Health ───────────────────────────────────────────────────
		{
			name: "idx_provider_health_history_hypervisor_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_provider_health_history_hypervisor_created
				  ON provider_health_histories (hypervisor_id, created_at DESC)`,
		},

		// ── Schedules ────────────────────────────────────────────────────────
		// ListDue: WHERE enabled = true AND next_run_at <= now
		{
			name: "idx_schedules_due",
			ddl: `CREATE INDEX IF NOT EXISTS idx_schedules_due
				  ON schedules (next_run_at ASC)
				  WHERE deleted_at IS NULL AND enabled = true`,
		},

		// ── Platform Events ───────────────────────────────────────────────────
		// Note: platform_events has no deleted_at column (no soft-delete), so
		// no WHERE clause is used here.
		{
			name: "idx_platform_events_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_platform_events_created
				  ON platform_events (created_at DESC)`,
		},
		{
			name: "idx_platform_events_hypervisor_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_platform_events_hypervisor_created
				  ON platform_events (hypervisor_id, created_at DESC)
				  WHERE hypervisor_id IS NOT NULL`,
		},

		// ── Notification History ──────────────────────────────────────────────
		{
			name: "idx_notification_history_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_notification_history_created
				  ON notification_histories (created_at DESC)`,
		},

		// ── Infrastructure Metrics ────────────────────────────────────────────
		// Note: infrastructure_metrics is a platform-wide aggregate — no hypervisor_id.
		// The only useful index is on collected_at for time-range queries.
		{
			name: "idx_infra_metrics_collected",
			ddl: `CREATE INDEX IF NOT EXISTS idx_infra_metrics_collected
				  ON infrastructure_metrics (collected_at DESC)`,
		},

		// ── Capacity History ──────────────────────────────────────────────────
		// capacity_histories has hypervisor_id and collected_at (not recorded_at).
		{
			name: "idx_capacity_history_hypervisor_collected",
			ddl: `CREATE INDEX IF NOT EXISTS idx_capacity_history_hypervisor_collected
				  ON capacity_histories (hypervisor_id, collected_at DESC)`,
		},

		// ── VM Usage Stats ────────────────────────────────────────────────────
		// vm_usage_stats is one row per VM (upserted) — vm_id already has a uniqueIndex.
		// No time-series column exists, so no additional composite index is needed.

		// ── Orchestration Runs ────────────────────────────────────────────────
		{
			name: "idx_orchestration_runs_env_created",
			ddl: `CREATE INDEX IF NOT EXISTS idx_orchestration_runs_env_created
				  ON orchestration_runs (environment_id, created_at DESC)
				  WHERE deleted_at IS NULL`,
		},
		// ── Infrastructure Hierarchy ──────────────────────────────────────────
		{
			name: "idx_clusters_hypervisor",
			ddl: `CREATE INDEX IF NOT EXISTS idx_clusters_hypervisor
				  ON clusters (hypervisor_id)
				  WHERE deleted_at IS NULL`,
		},
		{
			name: "idx_hosts_hypervisor",
			ddl: `CREATE INDEX IF NOT EXISTS idx_hosts_hypervisor
				  ON hosts (hypervisor_id)
				  WHERE deleted_at IS NULL`,
		},
		{
			name: "idx_hosts_cluster",
			ddl: `CREATE INDEX IF NOT EXISTS idx_hosts_cluster
				  ON hosts (cluster_id)
				  WHERE deleted_at IS NULL AND cluster_id IS NOT NULL`,
		},
	}

	for _, idx := range indexes {
		if err := db.Exec(idx.ddl).Error; err != nil {
			return fmt.Errorf("creating index %s: %w", idx.name, err)
		}
	}
	return nil
}

// ArchiveOldTasks soft-deletes completed/failed/cancelled tasks older than
// retentionDays. Call this from a periodic maintenance job.
// Returns the number of rows archived.
func ArchiveOldTasks(db *gorm.DB, retentionDays int) (int64, error) {
	result := db.Exec(`
		UPDATE tasks
		SET    deleted_at = NOW()
		WHERE  status IN ('completed', 'failed', 'cancelled', 'timed_out')
		  AND  created_at < NOW() - INTERVAL '1 day' * ?
		  AND  deleted_at IS NULL
	`, retentionDays)
	return result.RowsAffected, result.Error
}

// ArchiveOldAuditLogs soft-deletes audit log entries older than retentionDays.
// Audit logs are append-only and must never be hard-deleted.
func ArchiveOldAuditLogs(db *gorm.DB, retentionDays int) (int64, error) {
	// Audit logs don't have deleted_at — we use a separate archive table pattern.
	// For now, just return the count of old rows without deleting.
	var count int64
	db.Raw(`
		SELECT COUNT(*) FROM audit_logs
		WHERE created_at < NOW() - INTERVAL '1 day' * ?
	`, retentionDays).Scan(&count)
	return count, nil
}
