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

// staleInventoryThreshold is the age after which inventory is considered stale.
const staleInventoryThreshold = 2 * time.Hour

type providerHealthService struct {
	healthRepo port.ProviderHealthRepository
	hvRepo     port.HypervisorRepository
	auditRepo  port.AuditRepository
	registry   *provider.Registry
	log        logger.Logger
}

// NewProviderHealthService creates a new ProviderHealthService.
func NewProviderHealthService(
	healthRepo port.ProviderHealthRepository,
	hvRepo port.HypervisorRepository,
	auditRepo port.AuditRepository,
	registry *provider.Registry,
	log logger.Logger,
) port.ProviderHealthService {
	return &providerHealthService{
		healthRepo: healthRepo,
		hvRepo:     hvRepo,
		auditRepo:  auditRepo,
		registry:   registry,
		log:        log,
	}
}

func (s *providerHealthService) GetAll(ctx context.Context) ([]model.ProviderHealth, error) {
	// Fetch all existing health snapshots (keyed by hypervisor ID).
	snapshots, err := s.healthRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	snapshotByID := make(map[string]model.ProviderHealth, len(snapshots))
	for _, snap := range snapshots {
		snapshotByID[snap.HypervisorID.String()] = snap
	}

	// Fetch all registered hypervisors so every one appears in the response,
	// even if no health check has run yet.
	hvResult, err := s.hvRepo.List(ctx, port.Page{Number: 1, Size: 1000})
	if err != nil {
		// Fall back to whatever snapshots we have rather than returning an error.
		s.log.Warn("provider health: failed to list hypervisors, returning snapshots only", logger.Error(err))
		return snapshots, nil
	}

	result := make([]model.ProviderHealth, 0, len(hvResult.Items))
	for _, hv := range hvResult.Items {
		hvID, _ := uuid.Parse(hv.ID.String())
		if snap, ok := snapshotByID[hv.ID.String()]; ok {
			// Ensure the Hypervisor relation is populated for the frontend.
			if snap.Hypervisor.ID == uuid.Nil {
				snap.Hypervisor = hv
			}
			result = append(result, snap)
		} else {
			// No snapshot yet — synthesise a "pending" entry so the hypervisor
			// shows up in the UI immediately after registration.
			result = append(result, model.ProviderHealth{
				HypervisorID:        hvID,
				Hypervisor:          hv,
				Status:              model.HealthStatusUnknown,
				Online:              false,
				HealthScore:         0,
				InventoryStale:      true,
				InventoryAgeMinutes: -1,
			})
		}
	}
	return result, nil
}

func (s *providerHealthService) GetByHypervisorID(ctx context.Context, hypervisorID string) (*model.ProviderHealth, error) {
	return s.healthRepo.GetByHypervisorID(ctx, hypervisorID)
}

func (s *providerHealthService) GetHistory(ctx context.Context, hypervisorID string, limit int) ([]model.ProviderHealthHistory, error) {
	return s.healthRepo.GetHistory(ctx, hypervisorID, limit)
}

// RunCheck performs a connectivity ping for a single hypervisor, computes the
// health score, persists the snapshot, and appends a history row.
func (s *providerHealthService) RunCheck(ctx context.Context, hypervisorID string) (*model.ProviderHealth, error) {
	h, err := s.hvRepo.GetByID(ctx, hypervisorID)
	if err != nil {
		return nil, fmt.Errorf("hypervisor not found: %w", err)
	}

	// Load existing snapshot (may not exist yet)
	existing, _ := s.healthRepo.GetByHypervisorID(ctx, hypervisorID)

	snap := s.buildSnapshot(ctx, h, existing)

	if err := s.healthRepo.Upsert(ctx, snap); err != nil {
		s.log.Error("failed to upsert provider health",
			logger.String("hypervisor_id", hypervisorID),
			logger.Error(err),
		)
	}

	// Append history row for trend graphs
	hist := &model.ProviderHealthHistory{
		HypervisorID: snap.HypervisorID,
		Status:       snap.Status,
		Online:       snap.Online,
		LatencyMs:    snap.LatencyMs,
		HealthScore:  snap.HealthScore,
	}
	if err := s.healthRepo.AppendHistory(ctx, hist); err != nil {
		s.log.Warn("failed to append health history",
			logger.String("hypervisor_id", hypervisorID),
			logger.Error(err),
		)
	}

	return snap, nil
}

// buildSnapshot pings the provider and computes all health metrics.
func (s *providerHealthService) buildSnapshot(
	ctx context.Context,
	h *model.Hypervisor,
	existing *model.ProviderHealth,
) *model.ProviderHealth {
	hvID, _ := uuid.Parse(h.ID.String())
	now := time.Now().UTC()

	snap := &model.ProviderHealth{
		HypervisorID: hvID,
		LastCheckAt:  &now,
	}

	// Carry forward existing ID so the upsert updates the same row
	if existing != nil {
		snap.ID = existing.ID
		snap.AvgLatencyMs = existing.AvgLatencyMs
		snap.PeakLatencyMs = existing.PeakLatencyMs
		snap.ConsecutiveFails = existing.ConsecutiveFails
		snap.LastSyncAt = existing.LastSyncAt
		snap.LastSyncStatus = existing.LastSyncStatus
		snap.SyncFailures24h = existing.SyncFailures24h
		snap.VMCount = existing.VMCount
		snap.AuthFailures24h = existing.AuthFailures24h
	}

	// ── Connectivity ping ─────────────────────────────────────────────────────
	p, err := s.registry.Get(h.Provider)
	if err != nil {
		snap.Online = false
		snap.ConsecutiveFails++
	} else {
		creds, credErr := buildHypervisorCredentials(h)
		if credErr != nil {
			snap.Online = false
			snap.ConsecutiveFails++
		} else {
			pingStart := time.Now()
			connectErr := p.Connect(ctx, creds)
			if connectErr == nil {
				latency := float64(time.Since(pingStart).Milliseconds())
				_ = p.Disconnect(ctx)

				snap.Online = true
				snap.LatencyMs = latency
				snap.LastSeenAt = &now
				snap.ConsecutiveFails = 0

				// Rolling average (exponential moving average, α=0.3)
				if snap.AvgLatencyMs == 0 {
					snap.AvgLatencyMs = latency
				} else {
					snap.AvgLatencyMs = 0.7*snap.AvgLatencyMs + 0.3*latency
				}
				if latency > snap.PeakLatencyMs {
					snap.PeakLatencyMs = latency
				}
			} else {
				snap.Online = false
				snap.ConsecutiveFails++
				s.log.Warn("provider health check: ping failed",
					logger.String("hypervisor_id", h.ID.String()),
					logger.Error(connectErr),
				)
			}
		}
	}

	// ── Task failure rate (last 24 h) ─────────────────────────────────────────
	snap.TasksTotal24h, snap.TasksFailed24h = s.computeTaskStats(ctx, h.ID.String())
	if snap.TasksTotal24h > 0 {
		snap.TaskFailureRate = float64(snap.TasksFailed24h) / float64(snap.TasksTotal24h)
	}

	// ── Auth failure count (last 24 h) ────────────────────────────────────────
	snap.AuthFailures24h = s.computeAuthFailures(ctx, h.ID.String())

	// ── Inventory freshness ───────────────────────────────────────────────────
	if snap.LastSyncAt != nil {
		age := time.Since(*snap.LastSyncAt)
		snap.InventoryAgeMinutes = int(age.Minutes())
		snap.InventoryStale = age > staleInventoryThreshold
	} else {
		snap.InventoryStale = true
		snap.InventoryAgeMinutes = -1
	}

	// ── Health score (0–100) ──────────────────────────────────────────────────
	snap.HealthScore = computeHealthScore(snap)
	snap.Status = scoreToStatus(snap.HealthScore)

	return snap
}

// computeTaskStats counts total and failed tasks for a hypervisor in the last 24 h
// by querying the audit log for "execute" actions on the hypervisor.
func (s *providerHealthService) computeTaskStats(ctx context.Context, hypervisorID string) (total, failed int) {
	since := time.Now().UTC().Add(-24 * time.Hour)

	hvUUID, err := parseUUID(hypervisorID)
	if err != nil {
		return 0, 0
	}

	// Total execute actions on this hypervisor in the last 24 h
	filter := port.AuditFilter{
		Action:       model.AuditActionExecute,
		HypervisorID: &hvUUID,
		Since:        &since,
	}
	result, err := s.auditRepo.List(ctx, filter, port.Page{Number: 1, Size: 1})
	if err == nil {
		total = int(result.TotalItems)
	}

	// Failed execute actions
	successFalse := false
	filterFailed := filter
	filterFailed.SuccessOnly = &successFalse
	resultFailed, err := s.auditRepo.List(ctx, filterFailed, port.Page{Number: 1, Size: 1})
	if err == nil {
		failed = int(resultFailed.TotalItems)
	}

	return total, failed
}

// computeAuthFailures counts failed login attempts for a hypervisor in the last 24 h.
func (s *providerHealthService) computeAuthFailures(ctx context.Context, hypervisorID string) int {
	since := time.Now().UTC().Add(-24 * time.Hour)
	successFalse := false
	filter := port.AuditFilter{
		Action:      model.AuditActionLogin,
		Since:       &since,
		SuccessOnly: &successFalse,
	}
	hvUUID, err := parseUUID(hypervisorID)
	if err == nil {
		filter.HypervisorID = &hvUUID
	}
	result, err := s.auditRepo.List(ctx, filter, port.Page{Number: 1, Size: 1})
	if err != nil {
		return 0
	}
	return int(result.TotalItems)
}

// computeHealthScore returns a 0–100 score based on health metrics.
func computeHealthScore(snap *model.ProviderHealth) int {
	score := 100

	// Offline: heavy penalty
	if !snap.Online {
		score -= 40
		// Additional penalty for consecutive failures
		if snap.ConsecutiveFails >= 3 {
			score -= 20
		}
	}

	// High latency penalty (>500ms = -10, >1000ms = -20)
	if snap.AvgLatencyMs > 1000 {
		score -= 20
	} else if snap.AvgLatencyMs > 500 {
		score -= 10
	}

	// Task failure rate penalty
	if snap.TaskFailureRate > 0.5 {
		score -= 20
	} else if snap.TaskFailureRate > 0.2 {
		score -= 10
	}

	// Stale inventory penalty
	if snap.InventoryStale {
		score -= 10
	}

	// Auth failures penalty
	if snap.AuthFailures24h > 10 {
		score -= 10
	} else if snap.AuthFailures24h > 3 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}
	return score
}

// scoreToStatus converts a numeric health score to a HealthStatus.
func scoreToStatus(score int) model.HealthStatus {
	switch {
	case score >= 80:
		return model.HealthStatusHealthy
	case score >= 50:
		return model.HealthStatusDegraded
	default:
		return model.HealthStatusUnhealthy
	}
}

// parseUUID is a small helper to avoid importing uuid in multiple places.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
