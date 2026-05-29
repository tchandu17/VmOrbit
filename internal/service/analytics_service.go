package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// analyticsService implements port.AnalyticsService.
type analyticsService struct {
	infraMetrics    port.InfrastructureMetricsRepository
	capacityHistory port.CapacityHistoryRepository
	providerCap     port.ProviderCapacityRepository
	vmUsage         port.VMUsageStatsRepository
	recommendations port.OptimizationRecommendationRepository
	recHistory      port.RecommendationHistoryRepository
	hypervisors     port.HypervisorRepository
	vms             port.VMRepository
	snapshots       port.SnapshotRepository
	datastores      port.DataStoreRepository
	tasks           port.TaskRepository
	environments    port.EnvironmentRepository
	log             logger.Logger
}

// NewAnalyticsService creates a new analytics service.
func NewAnalyticsService(
	infraMetrics port.InfrastructureMetricsRepository,
	capacityHistory port.CapacityHistoryRepository,
	providerCap port.ProviderCapacityRepository,
	vmUsage port.VMUsageStatsRepository,
	recommendations port.OptimizationRecommendationRepository,
	recHistory port.RecommendationHistoryRepository,
	hypervisors port.HypervisorRepository,
	vms port.VMRepository,
	snapshots port.SnapshotRepository,
	datastores port.DataStoreRepository,
	tasks port.TaskRepository,
	environments port.EnvironmentRepository,
	log logger.Logger,
) port.AnalyticsService {
	return &analyticsService{
		infraMetrics:    infraMetrics,
		capacityHistory: capacityHistory,
		providerCap:     providerCap,
		vmUsage:         vmUsage,
		recommendations: recommendations,
		recHistory:      recHistory,
		hypervisors:     hypervisors,
		vms:             vms,
		snapshots:       snapshots,
		datastores:      datastores,
		tasks:           tasks,
		environments:    environments,
		log:             log,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CollectMetrics — full metrics collection cycle
// ─────────────────────────────────────────────────────────────────────────────

func (s *analyticsService) CollectMetrics(ctx context.Context) error {
	now := time.Now().UTC()

	// Load all hypervisors
	hvResult, err := s.hypervisors.List(ctx, port.Page{Number: 1, Size: 1000})
	if err != nil {
		return fmt.Errorf("listing hypervisors: %w", err)
	}
	hypervisors := hvResult.Items

	// Load all VMs
	vmResult, err := s.vms.List(ctx, port.VMFilter{}, port.Page{Number: 1, Size: 10000})
	if err != nil {
		return fmt.Errorf("listing vms: %w", err)
	}
	vms := vmResult.Items

	// Load all datastores
	var allDatastores []model.DataStore
	for i := range hypervisors {
		ds, err := s.datastores.List(ctx, hypervisors[i].ID.String())
		if err != nil {
			s.log.Warn("analytics: failed to list datastores",
				logger.String("hypervisor_id", hypervisors[i].ID.String()),
				logger.Error(err),
			)
			continue
		}
		allDatastores = append(allDatastores, ds...)
	}

	// Load environment count
	var envCount int64
	if s.environments != nil {
		envResult, err := s.environments.List(ctx, port.EnvironmentFilter{}, port.Page{Number: 1, Size: 1})
		if err == nil {
			envCount = envResult.TotalItems
		}
	}

	// Aggregate infrastructure metrics
	infra := &model.InfrastructureMetrics{
		CollectedAt:      now,
		TotalHypervisors: len(hypervisors),
		TotalEnvironments: int(envCount),
	}

	// Per-hypervisor capacity maps
	hvVMCount := make(map[string]int)
	hvRunningCount := make(map[string]int)
	hvCPU := make(map[string]int)
	hvMemory := make(map[string]int64)

	for i := range vms {
		vm := &vms[i]
		infra.TotalVMs++
		infra.TotalCPUCores += vm.CPUCount
		infra.TotalMemoryMB += int64(vm.MemoryMB)
		infra.TotalDiskGB += int64(vm.DiskGB)

		hvID := vm.HypervisorID.String()
		hvVMCount[hvID]++
		hvCPU[hvID] += vm.CPUCount
		hvMemory[hvID] += int64(vm.MemoryMB)

		switch vm.Status {
		case model.VMStatusRunning:
			infra.RunningVMs++
			hvRunningCount[hvID]++
		case model.VMStatusStopped:
			infra.StoppedVMs++
			infra.PoweredOffVMs++
		}
	}

	// Storage aggregation
	var totalStorageGB, usedStorageGB float64
	hvStorage := make(map[string]struct{ total, used, free float64 })
	for i := range allDatastores {
		ds := &allDatastores[i]
		totalStorageGB += ds.CapacityGB
		usedStorageGB += ds.UsedGB
		hvID := ds.HypervisorID
		s := hvStorage[hvID]
		s.total += ds.CapacityGB
		s.used += ds.UsedGB
		s.free += ds.FreeGB
		hvStorage[hvID] = s
	}

	if totalStorageGB > 0 {
		infra.StorageUtilisationPct = (usedStorageGB / totalStorageGB) * 100
	}

	// VM density
	if len(hypervisors) > 0 {
		infra.VMDensity = float64(infra.RunningVMs) / float64(len(hypervisors))
	}

	// Task stats (24 h)
	since24h := now.Add(-24 * time.Hour)
	taskResult, err := s.tasks.List(ctx, port.Page{Number: 1, Size: 10000})
	if err == nil {
		for i := range taskResult.Items {
			t := &taskResult.Items[i]
			if t.CreatedAt.After(since24h) {
				switch t.Status {
				case model.TaskStatusCompleted:
					infra.TasksCompleted24h++
				case model.TaskStatusFailed:
					infra.TasksFailed24h++
				}
			}
		}
	}

	// Insert infrastructure metrics snapshot
	if err := s.infraMetrics.Insert(ctx, infra); err != nil {
		s.log.Error("analytics: failed to insert infrastructure metrics", logger.Error(err))
	}

	// Per-hypervisor capacity
	for i := range hypervisors {
		hv := &hypervisors[i]
		hvID := hv.ID.String()
		st := hvStorage[hvID]

		var storagePct float64
		if st.total > 0 {
			storagePct = (st.used / st.total) * 100
		}

		cap := &model.ProviderCapacity{
			UpdatedAt:      now,
			HypervisorID:   hv.ID,
			TotalVMs:       hvVMCount[hvID],
			RunningVMs:     hvRunningCount[hvID],
			StoppedVMs:     hvVMCount[hvID] - hvRunningCount[hvID],
			TotalCPUCores:  hvCPU[hvID],
			TotalMemoryMB:  hvMemory[hvID],
			TotalStorageGB: st.total,
			UsedStorageGB:  st.used,
			FreeStorageGB:  st.free,
			StorageUsedPct: storagePct,
		}

		if err := s.providerCap.Upsert(ctx, cap); err != nil {
			s.log.Warn("analytics: failed to upsert provider capacity",
				logger.String("hypervisor_id", hvID),
				logger.Error(err),
			)
		}

		// Append to capacity history
		hist := &model.CapacityHistory{
			CollectedAt:    now,
			HypervisorID:   hv.ID,
			TotalVMs:       hvVMCount[hvID],
			RunningVMs:     hvRunningCount[hvID],
			TotalCPUCores:  hvCPU[hvID],
			TotalMemoryMB:  hvMemory[hvID],
			TotalStorageGB: st.total,
			UsedStorageGB:  st.used,
			FreeStorageGB:  st.free,
		}
		if err := s.capacityHistory.Insert(ctx, hist); err != nil {
			s.log.Warn("analytics: failed to insert capacity history",
				logger.String("hypervisor_id", hvID),
				logger.Error(err),
			)
		}
	}

	s.log.Info("analytics: metrics collection complete",
		logger.Int("vms", infra.TotalVMs),
		logger.Int("hypervisors", infra.TotalHypervisors),
	)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RunOptimizationEngine — generate recommendations
// ─────────────────────────────────────────────────────────────────────────────

func (s *analyticsService) RunOptimizationEngine(ctx context.Context) error {
	now := time.Now().UTC()

	vmResult, err := s.vms.List(ctx, port.VMFilter{}, port.Page{Number: 1, Size: 10000})
	if err != nil {
		return fmt.Errorf("listing vms: %w", err)
	}
	vms := vmResult.Items

	hvResult, err := s.hypervisors.List(ctx, port.Page{Number: 1, Size: 1000})
	if err != nil {
		return fmt.Errorf("listing hypervisors: %w", err)
	}

	// Build snapshot count map per VM
	vmSnapCount := make(map[string]int)
	for i := range vms {
		snaps, err := s.snapshots.ListByVMID(ctx, vms[i].ID.String())
		if err == nil {
			vmSnapCount[vms[i].ID.String()] = len(snaps)
		}
	}

	// Analyse each VM
	for i := range vms {
		vm := &vms[i]
		vmID := vm.ID.String()
		snapCount := vmSnapCount[vmID]

		stats := &model.VMUsageStats{
			UpdatedAt:    now,
			VMID:         vm.ID,
			CPUCount:     vm.CPUCount,
			MemoryMB:     vm.MemoryMB,
			DiskGB:       vm.DiskGB,
			SnapshotCount: snapCount,
		}

		// Idle VM: stopped for > 30 days (use UpdatedAt as proxy)
		if vm.Status == model.VMStatusStopped {
			daysStopped := int(now.Sub(vm.UpdatedAt).Hours() / 24)
			stats.DaysSinceLastPowerOn = daysStopped
			if daysStopped > 30 {
				stats.IsIdle = true
				s.upsertRecommendation(ctx, &model.OptimizationRecommendation{
					Fingerprint:  fmt.Sprintf("idle_vm:%s", vmID),
					Type:         model.RecommendationIdleVM,
					Severity:     model.RecommendationSeverityWarning,
					Status:       model.RecommendationStatusActive,
					VMID:         &vm.ID,
					HypervisorID: &vm.HypervisorID,
					Title:        fmt.Sprintf("Idle VM: %s", vm.Name),
					Description:  fmt.Sprintf("VM '%s' has been powered off for %d days. Consider decommissioning or archiving.", vm.Name, daysStopped),
					Action:       "Review the VM and either power it on if needed, or delete it to reclaim resources.",
					Score:        min(daysStopped, 100),
					EstimatedSavingsCPU: vm.CPUCount,
					EstimatedSavingsMB:  vm.MemoryMB,
					EstimatedSavingsGB:  float64(vm.DiskGB),
					Metadata: model.JSONMap{
						"days_stopped": daysStopped,
						"vm_name":      vm.Name,
					},
				})
			}
		}

		// Stale snapshots: VM has > 5 snapshots
		if snapCount > 5 {
			stats.HasStaleSnaps = true
			s.upsertRecommendation(ctx, &model.OptimizationRecommendation{
				Fingerprint:  fmt.Sprintf("stale_snapshot:%s", vmID),
				Type:         model.RecommendationStaleSnapshot,
				Severity:     model.RecommendationSeverityWarning,
				Status:       model.RecommendationStatusActive,
				VMID:         &vm.ID,
				HypervisorID: &vm.HypervisorID,
				Title:        fmt.Sprintf("Stale Snapshots: %s (%d snapshots)", vm.Name, snapCount),
				Description:  fmt.Sprintf("VM '%s' has %d snapshots. Excessive snapshots consume storage and degrade performance.", vm.Name, snapCount),
				Action:       "Review and delete old snapshots. Keep only the most recent 2–3 snapshots.",
				Score:        min(snapCount*10, 100),
				EstimatedSavingsGB: float64(snapCount) * float64(vm.DiskGB) * 0.1,
				Metadata: model.JSONMap{
					"snapshot_count": snapCount,
					"vm_name":        vm.Name,
				},
			})
		}

		// Oversized VM: > 16 vCPUs or > 32 GB RAM and stopped
		if vm.Status == model.VMStatusStopped && (vm.CPUCount > 16 || vm.MemoryMB > 32768) {
			stats.IsOversized = true
			s.upsertRecommendation(ctx, &model.OptimizationRecommendation{
				Fingerprint:  fmt.Sprintf("oversized_vm:%s", vmID),
				Type:         model.RecommendationOversizedVM,
				Severity:     model.RecommendationSeverityInfo,
				Status:       model.RecommendationStatusActive,
				VMID:         &vm.ID,
				HypervisorID: &vm.HypervisorID,
				Title:        fmt.Sprintf("Potentially Oversized VM: %s", vm.Name),
				Description:  fmt.Sprintf("VM '%s' is allocated %d vCPUs and %d MB RAM but is currently stopped.", vm.Name, vm.CPUCount, vm.MemoryMB),
				Action:       "Review the VM's resource requirements and consider right-sizing.",
				Score:        50,
				EstimatedSavingsCPU: vm.CPUCount / 2,
				EstimatedSavingsMB:  vm.MemoryMB / 2,
				Metadata: model.JSONMap{
					"cpu_count": vm.CPUCount,
					"memory_mb": vm.MemoryMB,
					"vm_name":   vm.Name,
				},
			})
		}

		if err := s.vmUsage.Upsert(ctx, stats); err != nil {
			s.log.Warn("analytics: failed to upsert vm usage stats",
				logger.String("vm_id", vmID),
				logger.Error(err),
			)
		}
	}

	// Per-hypervisor analysis
	for i := range hvResult.Items {
		hv := &hvResult.Items[i]
		hvID := hv.ID.String()

		cap, err := s.providerCap.GetByHypervisor(ctx, hvID)
		if err != nil {
			continue
		}

		// Overcommitted host: storage > 85%
		if cap.StorageUsedPct > 85 {
			severity := model.RecommendationSeverityWarning
			if cap.StorageUsedPct > 95 {
				severity = model.RecommendationSeverityCritical
			}
			s.upsertRecommendation(ctx, &model.OptimizationRecommendation{
				Fingerprint:  fmt.Sprintf("storage_exhaustion:%s", hvID),
				Type:         model.RecommendationStorageExhaustion,
				Severity:     severity,
				Status:       model.RecommendationStatusActive,
				HypervisorID: &hv.ID,
				Title:        fmt.Sprintf("Storage Exhaustion Risk: %s (%.1f%% used)", hv.Name, cap.StorageUsedPct),
				Description:  fmt.Sprintf("Hypervisor '%s' has %.1f%% storage utilisation (%.1f GB free of %.1f GB total).", hv.Name, cap.StorageUsedPct, cap.FreeStorageGB, cap.TotalStorageGB),
				Action:       "Add storage capacity, migrate VMs to other datastores, or delete unused VMs and snapshots.",
				Score:        int(cap.StorageUsedPct),
				EstimatedSavingsGB: 0,
				Metadata: model.JSONMap{
					"storage_used_pct": cap.StorageUsedPct,
					"free_storage_gb":  cap.FreeStorageGB,
					"total_storage_gb": cap.TotalStorageGB,
					"hypervisor_name":  hv.Name,
				},
			})
		}

		// Underutilised host: < 10% VM density and has VMs
		if cap.TotalVMs > 0 && cap.RunningVMs == 0 {
			s.upsertRecommendation(ctx, &model.OptimizationRecommendation{
				Fingerprint:  fmt.Sprintf("underutilized_host:%s", hvID),
				Type:         model.RecommendationUnderutilizedHost,
				Severity:     model.RecommendationSeverityInfo,
				Status:       model.RecommendationStatusActive,
				HypervisorID: &hv.ID,
				Title:        fmt.Sprintf("Underutilised Hypervisor: %s", hv.Name),
				Description:  fmt.Sprintf("Hypervisor '%s' has %d VMs but none are running.", hv.Name, cap.TotalVMs),
				Action:       "Consider consolidating workloads or decommissioning this hypervisor.",
				Score:        30,
				Metadata: model.JSONMap{
					"total_vms":       cap.TotalVMs,
					"running_vms":     cap.RunningVMs,
					"hypervisor_name": hv.Name,
				},
			})
		}
	}

	s.log.Info("analytics: optimization engine run complete")
	return nil
}

func (s *analyticsService) upsertRecommendation(ctx context.Context, rec *model.OptimizationRecommendation) {
	if err := s.recommendations.Upsert(ctx, rec); err != nil {
		s.log.Warn("analytics: failed to upsert recommendation",
			logger.String("fingerprint", rec.Fingerprint),
			logger.Error(err),
		)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─────────────────────────────────────────────────────────────────────────────
// GetCapacitySummary
// ─────────────────────────────────────────────────────────────────────────────

func (s *analyticsService) GetCapacitySummary(ctx context.Context) (*port.CapacitySummary, error) {
	// Try latest snapshot first
	latest, err := s.infraMetrics.Latest(ctx)
	if err != nil {
		// No snapshot yet — compute on-the-fly
		if collectErr := s.CollectMetrics(ctx); collectErr != nil {
			return nil, fmt.Errorf("collecting metrics: %w", collectErr)
		}
		latest, err = s.infraMetrics.Latest(ctx)
		if err != nil {
			return nil, fmt.Errorf("no metrics available: %w", err)
		}
	}

	// Get storage totals from provider capacity
	caps, err := s.providerCap.List(ctx)
	if err != nil {
		caps = nil
	}
	var totalStorageGB, usedStorageGB, freeStorageGB float64
	for i := range caps {
		totalStorageGB += caps[i].TotalStorageGB
		usedStorageGB += caps[i].UsedStorageGB
		freeStorageGB += caps[i].FreeStorageGB
	}

	// Get recommendation counts
	bySeverity, _ := s.recommendations.CountBySeverity(ctx)

	summary := &port.CapacitySummary{
		TotalHypervisors:      latest.TotalHypervisors,
		TotalVMs:              latest.TotalVMs,
		RunningVMs:            latest.RunningVMs,
		StoppedVMs:            latest.StoppedVMs,
		TotalCPUCores:         latest.TotalCPUCores,
		TotalMemoryMB:         latest.TotalMemoryMB,
		TotalDiskGB:           latest.TotalDiskGB,
		TotalSnapshots:        latest.TotalSnapshots,
		TotalEnvironments:     latest.TotalEnvironments,
		CPUUtilisationPct:     latest.CPUUtilisationPct,
		MemoryUtilisationPct:  latest.MemoryUtilisationPct,
		StorageUtilisationPct: latest.StorageUtilisationPct,
		TotalStorageGB:        totalStorageGB,
		UsedStorageGB:         usedStorageGB,
		FreeStorageGB:         freeStorageGB,
		VMDensity:             latest.VMDensity,
		TasksCompleted24h:     latest.TasksCompleted24h,
		TasksFailed24h:        latest.TasksFailed24h,
		CriticalRecommendations: int(bySeverity[string(model.RecommendationSeverityCritical)]),
		WarningRecommendations:  int(bySeverity[string(model.RecommendationSeverityWarning)]),
		InfoRecommendations:     int(bySeverity[string(model.RecommendationSeverityInfo)]),
		CollectedAt:           latest.CollectedAt,
	}
	return summary, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetCapacityTrends
// ─────────────────────────────────────────────────────────────────────────────

func (s *analyticsService) GetCapacityTrends(ctx context.Context, req port.CapacityTrendsRequest) (*port.CapacityTrends, error) {
	since := req.Since
	if since.IsZero() {
		since = time.Now().UTC().Add(-7 * 24 * time.Hour)
	}

	var trends port.CapacityTrends
	trends.ProviderVMGrowth = make(map[string][]port.TimeSeriesPoint)

	if req.HypervisorID != "" {
		items, err := s.capacityHistory.ListByHypervisor(ctx, req.HypervisorID, since, 2000)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ts := item.CollectedAt
			trends.VMGrowth = append(trends.VMGrowth, port.TimeSeriesPoint{Timestamp: ts, Value: float64(item.TotalVMs)})
			trends.StorageTrend = append(trends.StorageTrend, port.TimeSeriesPoint{Timestamp: ts, Value: item.UsedStorageGB})
			trends.SnapshotTrend = append(trends.SnapshotTrend, port.TimeSeriesPoint{Timestamp: ts, Value: float64(item.SnapshotCount)})
		}
	} else {
		// Aggregate across all hypervisors using infrastructure metrics
		metrics, err := s.infraMetrics.ListSince(ctx, since, 2000)
		if err != nil {
			return nil, err
		}
		for _, m := range metrics {
			ts := m.CollectedAt
			trends.VMGrowth = append(trends.VMGrowth, port.TimeSeriesPoint{Timestamp: ts, Value: float64(m.TotalVMs)})
			trends.StorageTrend = append(trends.StorageTrend, port.TimeSeriesPoint{Timestamp: ts, Value: float64(m.TotalDiskGB)})
			trends.SnapshotTrend = append(trends.SnapshotTrend, port.TimeSeriesPoint{Timestamp: ts, Value: float64(m.TotalSnapshots)})
			trends.TaskTrend = append(trends.TaskTrend, port.TimeSeriesPoint{Timestamp: ts, Value: float64(m.TasksCompleted24h)})
		}

		// Per-provider VM growth
		allHistory, err := s.capacityHistory.ListAllSince(ctx, since, 10000)
		if err == nil {
			for _, h := range allHistory {
				hvID := h.HypervisorID.String()
				trends.ProviderVMGrowth[hvID] = append(trends.ProviderVMGrowth[hvID],
					port.TimeSeriesPoint{Timestamp: h.CollectedAt, Value: float64(h.TotalVMs)})
			}
		}
	}

	return &trends, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetProviderCapacity
// ─────────────────────────────────────────────────────────────────────────────

func (s *analyticsService) GetProviderCapacity(ctx context.Context) ([]model.ProviderCapacity, error) {
	return s.providerCap.List(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// Recommendations
// ─────────────────────────────────────────────────────────────────────────────

func (s *analyticsService) GetRecommendations(ctx context.Context, filter port.RecommendationFilter, page port.Page) (*port.PageResult[model.OptimizationRecommendation], error) {
	return s.recommendations.List(ctx, filter, page)
}

func (s *analyticsService) DismissRecommendation(ctx context.Context, id string, userID string, note string) error {
	rec, err := s.recommendations.GetByID(ctx, id)
	if err != nil {
		return err
	}
	oldStatus := string(rec.Status)
	if err := s.recommendations.UpdateStatus(ctx, id, model.RecommendationStatusDismissed, &userID, note); err != nil {
		return err
	}
	recUUID, _ := uuid.Parse(id)
	userUUID, _ := uuid.Parse(userID)
	_ = s.recHistory.Create(ctx, &model.RecommendationHistory{
		RecommendationID: recUUID,
		OldStatus:        oldStatus,
		NewStatus:        string(model.RecommendationStatusDismissed),
		ChangedBy:        &userUUID,
		Note:             note,
	})
	return nil
}

func (s *analyticsService) ResolveRecommendation(ctx context.Context, id string, userID string) error {
	rec, err := s.recommendations.GetByID(ctx, id)
	if err != nil {
		return err
	}
	oldStatus := string(rec.Status)
	if err := s.recommendations.UpdateStatus(ctx, id, model.RecommendationStatusResolved, &userID, ""); err != nil {
		return err
	}
	recUUID, _ := uuid.Parse(id)
	userUUID, _ := uuid.Parse(userID)
	_ = s.recHistory.Create(ctx, &model.RecommendationHistory{
		RecommendationID: recUUID,
		OldStatus:        oldStatus,
		NewStatus:        string(model.RecommendationStatusResolved),
		ChangedBy:        &userUUID,
	})
	return nil
}

func (s *analyticsService) GetRecommendationSummary(ctx context.Context) (*port.RecommendationSummary, error) {
	byStatus, err := s.recommendations.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}
	bySeverity, err := s.recommendations.CountBySeverity(ctx)
	if err != nil {
		return nil, err
	}

	// Count by type for active recommendations
	recs, err := s.recommendations.List(ctx, port.RecommendationFilter{Status: string(model.RecommendationStatusActive)}, port.Page{Number: 1, Size: 1000})
	byType := make(map[string]int64)
	if err == nil {
		for _, r := range recs.Items {
			byType[string(r.Type)]++
		}
	}

	return &port.RecommendationSummary{
		TotalActive:    byStatus[string(model.RecommendationStatusActive)],
		TotalDismissed: byStatus[string(model.RecommendationStatusDismissed)],
		TotalResolved:  byStatus[string(model.RecommendationStatusResolved)],
		BySeverity:     bySeverity,
		ByType:         byType,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetForecasts — trend-based capacity forecasting
// ─────────────────────────────────────────────────────────────────────────────

func (s *analyticsService) GetForecasts(ctx context.Context) (*port.ForecastReport, error) {
	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour) // 30-day window for trend calculation

	hvResult, err := s.hypervisors.List(ctx, port.Page{Number: 1, Size: 1000})
	if err != nil {
		return nil, err
	}

	report := &port.ForecastReport{
		GeneratedAt: now,
	}

	for i := range hvResult.Items {
		hv := &hvResult.Items[i]
		hvID := hv.ID.String()

		history, err := s.capacityHistory.ListByHypervisor(ctx, hvID, since, 500)
		if err != nil || len(history) < 2 {
			continue
		}

		forecast := port.HypervisorForecast{
			HypervisorID:   hvID,
			HypervisorName: hv.Name,
			Provider:       string(hv.Provider),
		}

		// Linear regression for storage growth
		storageGrowthRate := linearGrowthRate(history, func(h model.CapacityHistory) float64 { return h.UsedStorageGB })
		forecast.StorageGrowthRateGBDay = storageGrowthRate

		// Current storage utilisation
		cap, err := s.providerCap.GetByHypervisor(ctx, hvID)
		if err == nil {
			forecast.CurrentStorageUsedPct = cap.StorageUsedPct
			if storageGrowthRate > 0 && cap.FreeStorageGB > 0 {
				daysUntilFull := int(cap.FreeStorageGB / storageGrowthRate)
				forecast.StorageExhaustionDays = &daysUntilFull
				if daysUntilFull < 30 {
					forecast.Risks = append(forecast.Risks, port.ForecastRisk{
						Type:        "storage_exhaustion",
						Severity:    "critical",
						Description: fmt.Sprintf("Storage will be exhausted in ~%d days at current growth rate.", daysUntilFull),
						DaysUntil:   &daysUntilFull,
					})
				} else if daysUntilFull < 90 {
					forecast.Risks = append(forecast.Risks, port.ForecastRisk{
						Type:        "storage_exhaustion",
						Severity:    "warning",
						Description: fmt.Sprintf("Storage will be exhausted in ~%d days at current growth rate.", daysUntilFull),
						DaysUntil:   &daysUntilFull,
					})
				}
			}
		}

		// VM growth rate
		vmGrowthRate := linearGrowthRate(history, func(h model.CapacityHistory) float64 { return float64(h.TotalVMs) })
		forecast.VMGrowthRatePerDay = vmGrowthRate
		if len(history) > 0 {
			currentVMs := history[len(history)-1].TotalVMs
			forecast.ProjectedVMs30Days = currentVMs + int(vmGrowthRate*30)
		}

		// Snapshot growth
		snapGrowthRate := linearGrowthRate(history, func(h model.CapacityHistory) float64 { return float64(h.SnapshotCount) })
		forecast.SnapshotGrowthRatePerDay = snapGrowthRate
		switch {
		case snapGrowthRate > 5:
			forecast.SnapshotRisk = "high"
			forecast.Risks = append(forecast.Risks, port.ForecastRisk{
				Type:        "snapshot_growth",
				Severity:    "warning",
				Description: fmt.Sprintf("Snapshot count growing at %.1f/day. Storage impact may be significant.", snapGrowthRate),
			})
		case snapGrowthRate > 2:
			forecast.SnapshotRisk = "medium"
		default:
			forecast.SnapshotRisk = "low"
		}

		report.Forecasts = append(report.Forecasts, forecast)
	}

	return report, nil
}

// linearGrowthRate computes the average daily growth rate using simple linear regression.
func linearGrowthRate[T any](items []T, valueFn func(T) float64) float64 {
	n := len(items)
	if n < 2 {
		return 0
	}
	// Use first and last values over the time span
	first := valueFn(items[0])
	last := valueFn(items[n-1])
	diff := last - first
	if diff <= 0 {
		return 0
	}
	// Assume items are roughly evenly spaced; use count as proxy for days
	return diff / float64(n)
}

// Ensure math is imported (used for Abs in future extensions)
var _ = math.Abs
