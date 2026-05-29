package policy

import (
	"context"
	"time"

	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ExpiryWorker periodically expires stale approval requests.
type ExpiryWorker struct {
	approvalSvc  port.ApprovalService
	interval     time.Duration
	log          logger.Logger
	stopCh       chan struct{}
}

// NewExpiryWorker creates a new expiry worker.
func NewExpiryWorker(approvalSvc port.ApprovalService, interval time.Duration, log logger.Logger) *ExpiryWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &ExpiryWorker{
		approvalSvc: approvalSvc,
		interval:    interval,
		log:         log,
		stopCh:      make(chan struct{}),
	}
}

// Start launches the expiry loop in a goroutine.
func (w *ExpiryWorker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop signals the worker to stop.
func (w *ExpiryWorker) Stop() {
	close(w.stopCh)
}

func (w *ExpiryWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			count, err := w.approvalSvc.ExpireStale(ctx)
			if err != nil {
				w.log.Error("approval expiry worker: failed to expire stale requests",
					logger.Error(err),
				)
				continue
			}
			if count > 0 {
				w.log.Info("approval expiry worker: expired stale requests",
					logger.Int("count", count),
				)
			}
		}
	}
}
