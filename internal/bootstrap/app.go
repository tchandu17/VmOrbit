package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vmOrbit/backend/internal/analytics"
	"github.com/vmOrbit/backend/internal/api"
	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/crypto"
	"github.com/vmOrbit/backend/internal/health"
	"github.com/vmOrbit/backend/internal/infrastructure/cache"
	"github.com/vmOrbit/backend/internal/infrastructure/database"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/internal/policy"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/internal/provider/esxi"
	"github.com/vmOrbit/backend/internal/provider/nutanix"
	"github.com/vmOrbit/backend/internal/provider/proxmox"
	"github.com/vmOrbit/backend/internal/provider/vmware"
	"github.com/vmOrbit/backend/internal/scheduler"
	"github.com/vmOrbit/backend/internal/service"
	"github.com/vmOrbit/backend/internal/task"
	"github.com/vmOrbit/backend/internal/websocket"
	"github.com/vmOrbit/backend/pkg/logger"
)

// Application is the root application container.
type Application struct {
	cfg              *config.Config
	log              logger.Logger
	httpServer       *http.Server
	taskEngine       *task.Engine
	wsHub            *websocket.Hub
	providerManager  *provider.Manager
	healthEngine     *health.Engine
	eventIntegration *service.EventIntegration
	schedulerEngine  *scheduler.Engine
	workflowEngine   *scheduler.WorkflowEngine
	analyticsEngine  *analytics.Engine
	approvalExpiry   *policy.ExpiryWorker
}

// NewApplication wires all dependencies and returns a ready Application.
func NewApplication(log logger.Logger) (*Application, error) {
	// ── Config ───────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	// Warn when running with the development encryption key
	if crypto.IsDevelopmentKey() {
		log.Warn("⚠️  VMORBIT_ENCRYPTION_KEY is not set — using insecure development key. Set this env var before production deployment.")
	}

	// ── Database ─────────────────────────────────────────────────────────────
	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("migration: %w", err)
	}

	// ── Cache ─────────────────────────────────────────────────────────────────
	redisClient, err := cache.NewRedisClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	cacheStore := cache.NewRedisCache(redisClient)

	// Dedicated Redis client for the task engine — workers use blocking BRPOP
	// which holds connections. A separate client prevents pool starvation for
	// the auth cache (refresh tokens, etc.).
	taskRedisCfg := cfg.Redis
	taskRedisCfg.PoolSize = cfg.TaskEngine.WorkerCount + 5 // workers + headroom
	taskRedisClient, err := cache.NewRedisClient(taskRedisCfg)
	if err != nil {
		return nil, fmt.Errorf("task redis: %w", err)
	}

	// ── Event Bus ─────────────────────────────────────────────────────────────
	eventBus := messaging.NewInMemoryEventBus()

	// ── WebSocket Hub ─────────────────────────────────────────────────────────
	wsHub := websocket.NewHub(log, eventBus)

	// ── Provider Registry ─────────────────────────────────────────────────────
	registry := provider.NewRegistry(log)
	registry.Register(vmware.NewProvider(cfg.Providers.VMware, log))
	registry.Register(esxi.NewProvider(cfg.Providers.VMware, log))
	registry.Register(proxmox.NewProvider(cfg.Providers.Proxmox, log))
	registry.Register(nutanix.NewProvider(cfg.Providers.Nutanix, log))

	// ── Provider Manager ──────────────────────────────────────────────────────
	providerManager := provider.NewManager(registry, provider.DefaultManagerConfig(), log)

	// ── Repositories ──────────────────────────────────────────────────────────
	repos := service.NewRepositories(db)

	// ── Services ──────────────────────────────────────────────────────────────
	services := service.NewServices(service.Deps{
		Repos:           repos,
		Cache:           cacheStore,
		RedisClient:     redisClient,
		Registry:        registry,
		ProviderManager: providerManager,
		EventBus:        eventBus,
		Log:             log,
		Config:          cfg,
	})

	// ── Task Engine ───────────────────────────────────────────────────────────
	taskEngine := task.NewEngine(task.EngineDeps{
		TaskRepo:         repos.Tasks,
		VMRepo:           repos.VMs,
		SnapshotRepo:     repos.Snapshots,
		HypervisorRepo:   repos.Hypervisors,
		ProvisioningRepo: repos.ProvisioningJobs,
		Registry:         registry,
		RedisClient:      taskRedisClient,
		Services:         services,
		EventBus:         eventBus,
		Log:              log,
		Config:           cfg.TaskEngine,
	})

	// ── Scheduler Engine ──────────────────────────────────────────────────────
	schedulerOps := scheduler.SchedulerOps{
		SyncInventory:  services.Hypervisors.SyncInventory,
		VMPowerOn:      services.VMs.PowerOn,
		VMPowerOff:     services.VMs.PowerOff,
		VMReboot:       services.VMs.Reboot,
		VMSnapshot:     services.VMs.CreateSnapshot,
		VMBulkPowerOn:  services.VMs.BulkPowerOn,
		VMBulkPowerOff: services.VMs.BulkPowerOff,
		VMBulkReboot:   services.VMs.BulkReboot,
		VMBulkSnapshot: services.VMs.BulkSnapshot,
	}
	schedulerEngine := scheduler.NewEngine(scheduler.EngineDeps{
		Schedules:          repos.Schedules,
		ScheduleExecutions: repos.ScheduleExecutions,
		Ops:                schedulerOps,
		Enqueue:            taskEngine.Enqueue,
		Log:                log,
		PollInterval:       30 * time.Second,
	})

	// ── Workflow Engine ───────────────────────────────────────────────────────
	workflowEngine := service.ExtractWorkflowEngine(services.Workflows)
	if workflowEngine != nil {
		workflowEngine.SetOps(scheduler.WorkflowOps{
			VMPowerOn:     services.VMs.PowerOn,
			VMPowerOff:    services.VMs.PowerOff,
			VMReboot:      services.VMs.Reboot,
			VMSnapshot:    services.VMs.CreateSnapshot,
			SyncInventory: services.Hypervisors.SyncInventory,
			ListVMsByTag:  services.Tags.ListVMsByTag,
			DispatchEvent: services.PlatformEvents.Dispatch,
		})
		workflowEngine.SetEnqueue(taskEngine.Enqueue)
	}

	// Inject scheduler engine into schedule service for TriggerNow.
	if ss := service.ExtractScheduleService(services.Schedules); ss != nil {
		ss.SetEngine(schedulerEngine)
	}

	// ── Health Engine ─────────────────────────────────────────────────────────
	healthEngine := health.NewEngine(repos.Hypervisors, services.ProviderHealth, 60*time.Second, log)

	// ── Analytics Engine ──────────────────────────────────────────────────────
	analyticsEngine := analytics.NewEngine(
		services.Analytics,
		15*time.Minute, // collect interval
		1*time.Hour,    // optimize interval
		log,
	)

	// ── Event Integration ─────────────────────────────────────────────────────
	eventIntegration := service.NewEventIntegration(eventBus, services.PlatformEvents, log)

	// ── Approval Expiry Worker ────────────────────────────────────────────────
	approvalExpiry := policy.NewExpiryWorker(services.Approvals, 5*time.Minute, log)

	// ── HTTP Router ───────────────────────────────────────────────────────────
	router := api.NewRouter(api.RouterDeps{
		Services:    services,
		TaskEngine:  taskEngine,
		WSHub:       wsHub,
		Log:         log,
		Config:      cfg,
		DB:          db,
		RedisClient: redisClient,
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &Application{
		cfg:              cfg,
		log:              log,
		httpServer:       httpServer,
		taskEngine:       taskEngine,
		wsHub:            wsHub,
		providerManager:  providerManager,
		healthEngine:     healthEngine,
		eventIntegration: eventIntegration,
		schedulerEngine:  schedulerEngine,
		workflowEngine:   workflowEngine,
		analyticsEngine:  analyticsEngine,
		approvalExpiry:   approvalExpiry,
	}, nil
}

// Start begins serving HTTP and background workers.
func (a *Application) Start(ctx context.Context) error {
	go a.wsHub.Run(ctx)
	go a.taskEngine.Start(ctx)
	a.providerManager.StartHealthChecks(ctx)
	a.healthEngine.Start(ctx)
	a.eventIntegration.Start()
	a.analyticsEngine.Start(ctx)
	a.approvalExpiry.Start(ctx)

	// Start scheduler and workflow engines.
	a.schedulerEngine.Start(ctx)
	if a.workflowEngine != nil {
		a.workflowEngine.Start()
	}

	a.log.Info("VmOrbit server starting", logger.String("addr", a.httpServer.Addr))
	if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// Stop gracefully shuts down all components.
func (a *Application) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	a.eventIntegration.Stop()
	a.taskEngine.Stop()
	a.schedulerEngine.Stop()
	a.analyticsEngine.Stop()
	a.approvalExpiry.Stop()
	if a.workflowEngine != nil {
		a.workflowEngine.Stop()
	}

	return a.httpServer.Shutdown(shutdownCtx)
}
