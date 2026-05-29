package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// HypervisorRepo is the GORM-backed hypervisor repository.
type HypervisorRepo struct{ db *gorm.DB }

// NewHypervisorRepo creates a new HypervisorRepo.
func NewHypervisorRepo(db *gorm.DB) *HypervisorRepo { return &HypervisorRepo{db: db} }

func (r *HypervisorRepo) Create(ctx context.Context, h *model.Hypervisor) error {
	return r.db.WithContext(ctx).Create(h).Error
}

func (r *HypervisorRepo) GetByID(ctx context.Context, id string) (*model.Hypervisor, error) {
	var h model.Hypervisor
	if err := r.db.WithContext(ctx).First(&h, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("hypervisor not found: %w", err)
	}
	return &h, nil
}

func (r *HypervisorRepo) Update(ctx context.Context, h *model.Hypervisor) error {
	return r.db.WithContext(ctx).Save(h).Error
}

func (r *HypervisorRepo) Delete(ctx context.Context, id string) error {
	// Since AutoMigrate is used (not the SQL migration), there are no actual
	// FK constraints in the DB. We still clean up child records explicitly
	// to keep the data consistent, then hard-delete the hypervisor.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Deletion order respects FK dependency chains.
		// All deletes use Unscoped() to bypass GORM soft-delete filters.
		//
		// Dependency tree (leaves first):
		//   snapshots          → vms, snapshots (self-ref parent_id)
		//   vm_dependencies    → vms
		//   environment_vms    → vms
		//   vm_usage_stats     → vms
		//   orchestration_steps → vms, tasks
		//   optimization_recommendations → vms, hypervisors
		//   provisioning_jobs  → vms (source/result), tasks, hypervisors
		//   schedule_executions → tasks
		//   tasks (sub-tasks)  → tasks (self-ref parent_task_id)
		//   task_logs          → tasks
		//   tasks              → hypervisors (nullified to preserve history)
		//   vms                → hypervisors
		//   vm_templates       → hypervisors
		//   data_stores        → hypervisors
		//   networks           → hypervisors
		//   provider_health_histories → hypervisors
		//   provider_healths   → hypervisors
		//   capacity_histories → hypervisors
		//   provider_capacities → hypervisors
		//   clusters           → hypervisors
		//   hosts              → hypervisors

		vmSubquery := `SELECT id FROM vms WHERE hypervisor_id = ?`
		taskSubquery := `SELECT id FROM tasks WHERE hypervisor_id = ?`

		// ── VM children ───────────────────────────────────────────────────────

		// 1. Snapshots (self-referencing parent_id — delete all at once; FK is nullable or same table)
		if err := tx.Unscoped().
			Where("vm_id IN ("+vmSubquery+")", id).
			Delete(&model.Snapshot{}).Error; err != nil {
			return fmt.Errorf("deleting snapshots: %w", err)
		}

		// 2. VM dependencies
		if err := tx.Exec(
			`DELETE FROM vm_dependencies WHERE source_vm_id IN (`+vmSubquery+`) OR target_vm_id IN (`+vmSubquery+`)`, id, id,
		).Error; err != nil {
			return fmt.Errorf("deleting vm_dependencies: %w", err)
		}

		// 3. Environment VM memberships (preserve the environment itself)
		if err := tx.Exec(
			`DELETE FROM environment_vms WHERE vm_id IN (`+vmSubquery+`)`, id,
		).Error; err != nil {
			return fmt.Errorf("deleting environment_vms: %w", err)
		}

		// 4. VM usage stats
		if err := tx.Unscoped().
			Where("vm_id IN ("+vmSubquery+")", id).
			Delete(&model.VMUsageStats{}).Error; err != nil {
			return fmt.Errorf("deleting vm_usage_stats: %w", err)
		}

		// ── Task children (before tasks are nullified) ─────────────────────────

		// 5. Orchestration steps — reference both tasks and vms
		if err := tx.Exec(
			`DELETE FROM orchestration_steps WHERE task_id IN (`+taskSubquery+`) OR vm_id IN (`+vmSubquery+`)`, id, id,
		).Error; err != nil {
			return fmt.Errorf("deleting orchestration_steps: %w", err)
		}

		// 6. Schedule executions
		if err := tx.Exec(
			`DELETE FROM schedule_executions WHERE task_id IN (`+taskSubquery+`)`, id,
		).Error; err != nil {
			return fmt.Errorf("deleting schedule_executions: %w", err)
		}

		// 7. Task logs
		if err := tx.Exec(
			`DELETE FROM task_logs WHERE task_id IN (`+taskSubquery+`)`, id,
		).Error; err != nil {
			return fmt.Errorf("deleting task_logs: %w", err)
		}

		// ── Hypervisor-direct children that also reference vms/tasks ──────────

		// 8. Optimization recommendations (references both hypervisor_id and vm_id)
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.OptimizationRecommendation{}).Error; err != nil {
			return fmt.Errorf("deleting optimization_recommendations: %w", err)
		}

		// 9. Provisioning jobs (references hypervisor_id, source_vm_id, result_vm_id, task_id)
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.ProvisioningJob{}).Error; err != nil {
			return fmt.Errorf("deleting provisioning_jobs: %w", err)
		}

		// ── Tasks — nullify to preserve history ───────────────────────────────

		// 10. Nullify sub-task parent references within this hypervisor's tasks
		if err := tx.Exec(
			`UPDATE tasks SET parent_task_id = NULL WHERE parent_task_id IN (`+taskSubquery+`)`, id,
		).Error; err != nil {
			return fmt.Errorf("nullifying sub-task parent_task_id: %w", err)
		}

		// 11. Nullify hypervisor_id on tasks (preserve task history)
		if err := tx.Exec(
			`UPDATE tasks SET hypervisor_id = NULL WHERE hypervisor_id = ?`, id,
		).Error; err != nil {
			return fmt.Errorf("nullifying task hypervisor_id: %w", err)
		}

		// ── VMs and direct hypervisor resources ───────────────────────────────

		// 12. VMs
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.VM{}).Error; err != nil {
			return fmt.Errorf("deleting vms: %w", err)
		}

		// 13. VM templates
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.VMTemplate{}).Error; err != nil {
			return fmt.Errorf("deleting vm_templates: %w", err)
		}

		// 14. DataStores
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.DataStore{}).Error; err != nil {
			return fmt.Errorf("deleting data_stores: %w", err)
		}

		// 15. Networks
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.Network{}).Error; err != nil {
			return fmt.Errorf("deleting networks: %w", err)
		}

		// ── Observability — nullify (append-only, must not be deleted) ─────────

		// 16. WebSocket events
		if err := tx.Model(&model.WebSocketEvent{}).
			Where("hypervisor_id = ?", id).
			Update("hypervisor_id", nil).Error; err != nil {
			return fmt.Errorf("nullifying web_socket_events hypervisor_id: %w", err)
		}

		// 17. Audit logs
		if err := tx.Exec(
			`UPDATE audit_logs SET hypervisor_id = NULL WHERE hypervisor_id = ?`, id,
		).Error; err != nil {
			return fmt.Errorf("nullifying audit_logs hypervisor_id: %w", err)
		}

		// ── Analytics / health data ────────────────────────────────────────────

		// 18. Provider health history
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.ProviderHealthHistory{}).Error; err != nil {
			return fmt.Errorf("deleting provider_health_histories: %w", err)
		}

		// 19. Provider health
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.ProviderHealth{}).Error; err != nil {
			return fmt.Errorf("deleting provider_healths: %w", err)
		}

		// 20. Capacity histories
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.CapacityHistory{}).Error; err != nil {
			return fmt.Errorf("deleting capacity_histories: %w", err)
		}

		// 21. Provider capacities
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.ProviderCapacity{}).Error; err != nil {
			return fmt.Errorf("deleting provider_capacities: %w", err)
		}

		// ── Infrastructure hierarchy ───────────────────────────────────────────

		// 22. Hosts
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.Host{}).Error; err != nil {
			return fmt.Errorf("deleting hosts: %w", err)
		}

		// 23. Clusters
		if err := tx.Unscoped().
			Where("hypervisor_id = ?", id).
			Delete(&model.Cluster{}).Error; err != nil {
			return fmt.Errorf("deleting clusters: %w", err)
		}

		// ── Finally, the hypervisor itself ─────────────────────────────────────

		// 24. Hard-delete the hypervisor (Unscoped bypasses soft-delete)
		if err := tx.Unscoped().
			Where("id = ?", id).
			Delete(&model.Hypervisor{}).Error; err != nil {
			return fmt.Errorf("deleting hypervisor: %w", err)
		}

		return nil
	})
}

func (r *HypervisorRepo) List(ctx context.Context, page port.Page) (*port.PageResult[model.Hypervisor], error) {
	var items []model.Hypervisor
	var total int64

	base := r.db.WithContext(ctx).Model(&model.Hypervisor{})

	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := r.db.WithContext(ctx).
		Model(&model.Hypervisor{}).
		Offset(offset).
		Limit(page.Size).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.Hypervisor]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *HypervisorRepo) UpdateConnectionStatus(ctx context.Context, id string, status model.ConnectionStatus) error {
	return r.db.WithContext(ctx).
		Model(&model.Hypervisor{}).
		Where("id = ?", id).
		Update("connection_status", status).Error
}
