package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Per-provider concurrency limiter
// ─────────────────────────────────────────────────────────────────────────────

// providerSemaphore limits how many workers can concurrently execute tasks
// against the same hypervisor. This prevents a single vCenter or Proxmox node
// from being overwhelmed by parallel API calls.
type providerSemaphore struct {
	mu   sync.Mutex
	sems map[string]chan struct{}
	max  int
}

func newProviderSemaphore(maxConcurrent int) *providerSemaphore {
	return &providerSemaphore{
		sems: make(map[string]chan struct{}),
		max:  maxConcurrent,
	}
}

func (ps *providerSemaphore) sem(hypervisorID string) chan struct{} {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if _, ok := ps.sems[hypervisorID]; !ok {
		ps.sems[hypervisorID] = make(chan struct{}, ps.max)
	}
	return ps.sems[hypervisorID]
}

// Acquire blocks until a slot is available for the given hypervisor, or the
// context is cancelled. Returns a release function that must be deferred.
func (ps *providerSemaphore) Acquire(ctx context.Context, hypervisorID string) (func(), error) {
	if hypervisorID == "" {
		return func() {}, nil // no hypervisor scoping needed
	}
	sem := ps.sem(hypervisorID)
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return func() {}, ctx.Err()
	}
}

// TryAcquire attempts a non-blocking acquire. Returns false if all slots are busy.
func (ps *providerSemaphore) TryAcquire(hypervisorID string) (func(), bool) {
	if hypervisorID == "" {
		return func() {}, true
	}
	sem := ps.sem(hypervisorID)
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, true
	default:
		return func() {}, false
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Task deduplication
// ─────────────────────────────────────────────────────────────────────────────

// deduplicator prevents the same logical operation from being enqueued twice
// within a short window. The dedup key is (task_type + hypervisor_id) for
// inventory syncs, and (task_type + vm_id) for VM operations.
type deduplicator struct {
	mu      sync.Mutex
	inflight map[string]time.Time
	window  time.Duration
}

func newDeduplicator(window time.Duration) *deduplicator {
	d := &deduplicator{
		inflight: make(map[string]time.Time),
		window:   window,
	}
	go d.cleanup()
	return d
}

func (d *deduplicator) cleanup() {
	ticker := time.NewTicker(d.window * 2)
	defer ticker.Stop()
	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for k, t := range d.inflight {
			if now.Sub(t) > d.window {
				delete(d.inflight, k)
			}
		}
		d.mu.Unlock()
	}
}

// IsDuplicate returns true if an identical operation was recently enqueued.
// If not a duplicate, it records the key and returns false.
func (d *deduplicator) IsDuplicate(taskType model.TaskType, scopeID string) bool {
	if scopeID == "" {
		return false
	}
	key := fmt.Sprintf("%s:%s", taskType, scopeID)
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.inflight[key]; ok && time.Since(t) < d.window {
		return true
	}
	d.inflight[key] = time.Now()
	return false
}

// Release removes the dedup key early (e.g. when a task completes or fails).
func (d *deduplicator) Release(taskType model.TaskType, scopeID string) {
	if scopeID == "" {
		return
	}
	key := fmt.Sprintf("%s:%s", taskType, scopeID)
	d.mu.Lock()
	delete(d.inflight, key)
	d.mu.Unlock()
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue backpressure
// ─────────────────────────────────────────────────────────────────────────────

// backpressureGuard rejects new enqueue requests when the queue depth exceeds
// a high-water mark. This prevents unbounded task accumulation during storms.
type backpressureGuard struct {
	mu        sync.RWMutex
	queueSize int
	hwm       int // high-water mark (reject above this)
}

func newBackpressureGuard(hwm int) *backpressureGuard {
	return &backpressureGuard{hwm: hwm}
}

// SetQueueSize updates the current queue depth (called periodically by the engine).
func (b *backpressureGuard) SetQueueSize(n int) {
	b.mu.Lock()
	b.queueSize = n
	b.mu.Unlock()
}

// Allow returns true if the queue is below the high-water mark.
func (b *backpressureGuard) Allow() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.queueSize < b.hwm
}

// QueueDepth returns the current queue depth.
func (b *backpressureGuard) QueueDepth() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.queueSize
}
