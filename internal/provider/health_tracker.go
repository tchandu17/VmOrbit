package provider

import (
	"context"
	"sync"
	"time"

	"github.com/vmOrbit/backend/pkg/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// ProviderHealthTracker
// ─────────────────────────────────────────────────────────────────────────────

// HealthStats holds rolling statistics for a single provider/hypervisor.
type HealthStats struct {
	HypervisorID    string
	TotalCalls      int64
	SuccessCalls    int64
	FailureCalls    int64
	ConsecFailures  int
	LastFailureAt   time.Time
	LastSuccessAt   time.Time
	AvgLatencyMs    float64
	latencySum      float64
	latencyCount    int64
}

// SuccessRate returns the percentage of successful calls (0–100).
func (h *HealthStats) SuccessRate() float64 {
	if h.TotalCalls == 0 {
		return 100
	}
	return float64(h.SuccessCalls) / float64(h.TotalCalls) * 100
}

// ProviderHealthTracker tracks per-hypervisor call success/failure rates
// and integrates with the circuit breaker registry in the middleware package.
type ProviderHealthTracker struct {
	mu    sync.RWMutex
	stats map[string]*HealthStats
	log   logger.Logger
}

// NewProviderHealthTracker creates a new tracker.
func NewProviderHealthTracker(log logger.Logger) *ProviderHealthTracker {
	return &ProviderHealthTracker{
		stats: make(map[string]*HealthStats),
		log:   log,
	}
}

// RecordSuccess records a successful provider call and resets the failure counter.
func (t *ProviderHealthTracker) RecordSuccess(hypervisorID string, latencyMs float64) {
	t.mu.Lock()
	s := t.getOrCreate(hypervisorID)
	s.TotalCalls++
	s.SuccessCalls++
	s.ConsecFailures = 0
	s.LastSuccessAt = time.Now()
	s.latencySum += latencyMs
	s.latencyCount++
	if s.latencyCount > 0 {
		s.AvgLatencyMs = s.latencySum / float64(s.latencyCount)
	}
	t.mu.Unlock()
}

// RecordFailure records a failed provider call and may open the circuit breaker.
func (t *ProviderHealthTracker) RecordFailure(hypervisorID string) {
	t.mu.Lock()
	s := t.getOrCreate(hypervisorID)
	s.TotalCalls++
	s.FailureCalls++
	s.ConsecFailures++
	s.LastFailureAt = time.Now()
	consecFails := s.ConsecFailures
	t.mu.Unlock()

	if consecFails >= 3 {
		t.log.Warn("provider consecutive failures threshold reached",
			logger.String("hypervisor_id", hypervisorID),
			logger.Int("consecutive_failures", consecFails),
		)
	}
}

// Stats returns a copy of the health stats for a hypervisor.
func (t *ProviderHealthTracker) Stats(hypervisorID string) HealthStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if s, ok := t.stats[hypervisorID]; ok {
		return *s
	}
	return HealthStats{HypervisorID: hypervisorID}
}

// AllStats returns a snapshot of all tracked hypervisors.
func (t *ProviderHealthTracker) AllStats() []HealthStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]HealthStats, 0, len(t.stats))
	for _, s := range t.stats {
		out = append(out, *s)
	}
	return out
}

// Reset clears stats for a hypervisor (e.g. after a successful reconnect).
func (t *ProviderHealthTracker) Reset(hypervisorID string) {
	t.mu.Lock()
	delete(t.stats, hypervisorID)
	t.mu.Unlock()
}

func (t *ProviderHealthTracker) getOrCreate(hypervisorID string) *HealthStats {
	if s, ok := t.stats[hypervisorID]; ok {
		return s
	}
	s := &HealthStats{HypervisorID: hypervisorID}
	t.stats[hypervisorID] = s
	return s
}

// ─────────────────────────────────────────────────────────────────────────────
// Timed provider call helper
// ─────────────────────────────────────────────────────────────────────────────

// TrackCall wraps a provider operation, recording success/failure and latency.
// Usage:
//
//	err := tracker.TrackCall(ctx, hypervisorID, func() error {
//	    return provider.PowerOn(ctx, vmID)
//	})
func (t *ProviderHealthTracker) TrackCall(_ context.Context, hypervisorID string, fn func() error) error {
	start := time.Now()
	err := fn()
	latencyMs := float64(time.Since(start).Milliseconds())
	if err != nil {
		t.RecordFailure(hypervisorID)
	} else {
		t.RecordSuccess(hypervisorID, latencyMs)
	}
	return err
}
