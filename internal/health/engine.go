// Package health provides the periodic provider health-check engine.
// It runs on a configurable interval, checks every registered hypervisor,
// and persists the results via ProviderHealthService.
package health

import (
	"context"
	"time"

	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

const defaultCheckInterval = 60 * time.Second

// Engine runs periodic health checks for all registered hypervisors.
type Engine struct {
	hvRepo      port.HypervisorRepository
	healthSvc   port.ProviderHealthService
	interval    time.Duration
	log         logger.Logger
}

// NewEngine creates a new health check engine.
// interval controls how often checks run; pass 0 to use the default (60 s).
func NewEngine(
	hvRepo port.HypervisorRepository,
	healthSvc port.ProviderHealthService,
	interval time.Duration,
	log logger.Logger,
) *Engine {
	if interval <= 0 {
		interval = defaultCheckInterval
	}
	return &Engine{
		hvRepo:    hvRepo,
		healthSvc: healthSvc,
		interval:  interval,
		log:       log,
	}
}

// Start launches the background check loop. Cancel ctx to stop.
func (e *Engine) Start(ctx context.Context) {
	go e.run(ctx)
}

func (e *Engine) run(ctx context.Context) {
	// Run an initial check immediately on startup
	e.checkAll(ctx)

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.checkAll(ctx)
		}
	}
}

func (e *Engine) checkAll(ctx context.Context) {
	result, err := e.hvRepo.List(ctx, port.Page{Number: 1, Size: 1000})
	if err != nil {
		e.log.Error("health engine: failed to list hypervisors", logger.Error(err))
		return
	}

	for _, hv := range result.Items {
		hvID := hv.ID.String()
		if _, err := e.healthSvc.RunCheck(ctx, hvID); err != nil {
			e.log.Warn("health engine: check failed",
				logger.String("hypervisor_id", hvID),
				logger.Error(err),
			)
		}
	}
}
