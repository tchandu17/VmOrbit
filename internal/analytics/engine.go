// Package analytics provides the background analytics collection engine.
// It runs on a configurable interval and calls the AnalyticsService to
// collect metrics and run the optimization engine.
package analytics

import (
	"context"
	"sync"
	"time"

	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// Engine is the background analytics collection engine.
type Engine struct {
	svc             port.AnalyticsService
	collectInterval time.Duration
	optimizeInterval time.Duration
	log             logger.Logger
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewEngine creates a new analytics engine.
func NewEngine(svc port.AnalyticsService, collectInterval, optimizeInterval time.Duration, log logger.Logger) *Engine {
	if collectInterval <= 0 {
		collectInterval = 15 * time.Minute
	}
	if optimizeInterval <= 0 {
		optimizeInterval = 1 * time.Hour
	}
	return &Engine{
		svc:              svc,
		collectInterval:  collectInterval,
		optimizeInterval: optimizeInterval,
		log:              log,
		stopCh:           make(chan struct{}),
	}
}

// Start launches the background collection goroutines.
func (e *Engine) Start(ctx context.Context) {
	e.log.Info("analytics engine starting",
		logger.String("collect_interval", e.collectInterval.String()),
		logger.String("optimize_interval", e.optimizeInterval.String()),
	)

	// Run an initial collection immediately on startup.
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		if err := e.svc.CollectMetrics(ctx); err != nil {
			e.log.Warn("analytics: initial metrics collection failed", logger.Error(err))
		}
		if err := e.svc.RunOptimizationEngine(ctx); err != nil {
			e.log.Warn("analytics: initial optimization run failed", logger.Error(err))
		}
	}()

	// Periodic metrics collection
	e.wg.Add(1)
	go e.runLoop(ctx, e.collectInterval, "metrics", func(ctx context.Context) error {
		return e.svc.CollectMetrics(ctx)
	})

	// Periodic optimization engine
	e.wg.Add(1)
	go e.runLoop(ctx, e.optimizeInterval, "optimization", func(ctx context.Context) error {
		return e.svc.RunOptimizationEngine(ctx)
	})
}

// Stop signals the engine to stop and waits for goroutines to finish.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	e.log.Info("analytics engine stopped")
}

func (e *Engine) runLoop(ctx context.Context, interval time.Duration, name string, fn func(context.Context) error) {
	defer e.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				e.log.Warn("analytics engine: job failed",
					logger.String("job", name),
					logger.Error(err),
				)
			} else {
				e.log.Debug("analytics engine: job complete", logger.String("job", name))
			}
		}
	}
}
