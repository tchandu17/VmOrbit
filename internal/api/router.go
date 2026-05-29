package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/api/handler"
	"github.com/vmOrbit/backend/internal/api/middleware"
	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/service"
	"github.com/vmOrbit/backend/internal/task"
	"github.com/vmOrbit/backend/internal/websocket"
	"github.com/vmOrbit/backend/pkg/logger"
)

// RouterDeps carries all dependencies needed to build the HTTP router.
type RouterDeps struct {
	Services    *service.Services
	TaskEngine  *task.Engine
	WSHub       *websocket.Hub
	Log         logger.Logger
	Config      *config.Config
	DB          *gorm.DB
	RedisClient *redis.Client
}

// NewRouter builds and returns the Gin engine with all routes registered.
func NewRouter(deps RouterDeps) http.Handler {
	if deps.Config.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// ── Global middleware ────────────────────────────────────────────────────
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(deps.Log))
	r.Use(middleware.Recovery(deps.Log))
	r.Use(middleware.CORS(deps.Config.Server.CORSOrigins))
	r.Use(middleware.GlobalRateLimit(6000))  // 100 req/s global cap
	r.Use(middleware.IPRateLimit(300))       // 5 req/s per IP

	// ── Operational probes (unauthenticated) ─────────────────────────────────
	opsHandler := handler.NewOpsHandler(deps.DB, deps.RedisClient, deps.Log, "1.0.0")
	r.GET("/health", opsHandler.Liveness)   // liveness probe
	r.GET("/ready", opsHandler.Readiness)   // readiness probe
	r.GET("/status", opsHandler.Status)     // extended status

	if deps.Config.Metrics.Enabled {
		r.GET(deps.Config.Metrics.Path, gin.WrapH(promhttp.Handler()))
	}

	// ── Maintenance mode ──────────────────────────────────────────────────────
	maintenanceState := handler.NewMaintenanceState()
	maintenanceHandler := handler.NewMaintenanceHandler(maintenanceState)
	r.Use(handler.MaintenanceMiddleware(maintenanceState))

	// ── Handlers ─────────────────────────────────────────────────────────────
	authHandler := handler.NewAuthHandler(deps.Services.Auth, deps.Services.Users, deps.Log)
	userHandler := handler.NewUserHandler(deps.Services.Users, deps.Log)
	roleHandler := handler.NewRoleHandler(deps.Services.Roles, deps.Services.Permissions, deps.Log)
	hypervisorHandler := handler.NewHypervisorHandler(deps.Services.Hypervisors, deps.TaskEngine.Enqueue, deps.Log)
	vmHandler := handler.NewVMHandler(deps.Services.VMs, deps.Services.Tasks, deps.Services.Audit, deps.TaskEngine.Enqueue, deps.Log)
	taskHandler := handler.NewTaskHandler(deps.Services.Tasks, deps.Log)
	auditHandler := handler.NewAuditHandler(deps.Services.Audit, deps.Log)
	consoleHandler := handler.NewConsoleHandler(deps.Services.Console, deps.Log)
	consoleProxyHandler := handler.NewConsoleProxyHandler(deps.Services.Console, deps.Log)
	wsHandler := handler.NewWSHandler(deps.WSHub, deps.Services.Auth, deps.Log)
	tagHandler := handler.NewTagHandler(deps.Services.Tags, deps.Log)
	providerHealthHandler := handler.NewProviderHealthHandler(deps.Services.ProviderHealth, deps.Log)
	eventHandler := handler.NewEventHandler(deps.Services.PlatformEvents, deps.Log)
	notificationHandler := handler.NewNotificationHandler(
		deps.Services.NotificationChannels,
		deps.Services.NotificationRules,
		deps.Services.NotificationHistory,
		deps.Log,
	)
	scheduleHandler := handler.NewScheduleHandler(deps.Services.Schedules, deps.Log)
	workflowHandler := handler.NewWorkflowHandler(deps.Services.Workflows, deps.Log)
	templateHandler := handler.NewTemplateHandler(deps.Services.Templates, deps.TaskEngine.Enqueue, deps.Log)
	provisioningHandler := handler.NewProvisioningHandler(deps.Services.Provisioning, deps.TaskEngine.Enqueue, deps.Log)
	environmentHandler := handler.NewEnvironmentHandler(deps.Services.Environments, deps.TaskEngine.Enqueue, deps.Log)
	analyticsHandler := handler.NewAnalyticsHandler(deps.Services.Analytics, deps.Log)
	policyHandler := handler.NewPolicyHandler(deps.Services.Policies, deps.Log)
	approvalHandler := handler.NewApprovalHandler(deps.Services.Approvals, deps.Log)

	// System health handler — provides deep metrics for the health dashboard.
	systemHealthHandler := handler.NewSystemHealthHandler(
		deps.DB, deps.RedisClient, deps.Services.Tasks, deps.Log,
	)

	// Infrastructure hierarchy handler
	infraHandler := handler.NewInfrastructureHandler(deps.Services.Infrastructure, deps.Log)

	// ── Auth routes (public) ─────────────────────────────────────────────────
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
		auth.POST("/logout", authHandler.Logout)
	}

	// ── Protected routes ─────────────────────────────────────────────────────
	authMiddleware := middleware.Auth(deps.Services.Auth)
	userRateLimit := middleware.UserRateLimit(600) // 10 req/s per user

	v1 := r.Group("/api/v1", authMiddleware, userRateLimit)
	{
		// Current user profile
		v1.GET("/auth/me", authHandler.Me)

		// Users
		users := v1.Group("/users")
		{
			users.GET("", middleware.RequirePermission("user", "read"), userHandler.List)
			users.POST("", middleware.RequirePermission("user", "write"), userHandler.Create)
			users.GET("/:id", middleware.RequirePermission("user", "read"), userHandler.GetByID)
			users.PUT("/:id", middleware.RequirePermission("user", "write"), userHandler.Update)
			users.DELETE("/:id", middleware.RequirePermission("user", "delete"), userHandler.Delete)
			users.PUT("/:id/password", middleware.RequirePermission("user", "write"), userHandler.ChangePassword)
			users.GET("/:id/permissions", middleware.RequirePermission("user", "read"), userHandler.GetPermissions)
			users.POST("/:id/roles/:roleId", middleware.RequirePermission("user", "write"), userHandler.AssignRole)
			users.DELETE("/:id/roles/:roleId", middleware.RequirePermission("user", "write"), userHandler.RevokeRole)
		}

		// Roles
		roles := v1.Group("/roles")
		{
			roles.GET("", middleware.RequirePermission("role", "read"), roleHandler.ListRoles)
			roles.POST("", middleware.RequirePermission("role", "write"), roleHandler.CreateRole)
			roles.GET("/:id", middleware.RequirePermission("role", "read"), roleHandler.GetRole)
			roles.PUT("/:id", middleware.RequirePermission("role", "write"), roleHandler.UpdateRole)
			roles.DELETE("/:id", middleware.RequirePermission("role", "delete"), roleHandler.DeleteRole)
			roles.POST("/:id/permissions/:permissionId", middleware.RequirePermission("role", "write"), roleHandler.AssignPermission)
			roles.DELETE("/:id/permissions/:permissionId", middleware.RequirePermission("role", "write"), roleHandler.RevokePermission)
			roles.PUT("/:id/permissions", middleware.RequirePermission("role", "write"), roleHandler.SetPermissions)
		}

		// Permissions
		permissions := v1.Group("/permissions")
		{
			permissions.GET("", middleware.RequirePermission("role", "read"), roleHandler.ListPermissions)
			permissions.POST("", middleware.RequirePermission("role", "write"), roleHandler.CreatePermission)
			permissions.DELETE("/:id", middleware.RequirePermission("role", "delete"), roleHandler.DeletePermission)
		}

		// Hypervisors
		hypervisors := v1.Group("/hypervisors")
		{
			hypervisors.GET("", middleware.RequirePermission("hypervisor", "read"), hypervisorHandler.List)
			hypervisors.POST("", middleware.RequirePermission("hypervisor", "write"), hypervisorHandler.Register)
			hypervisors.GET("/:id", middleware.RequirePermission("hypervisor", "read"), hypervisorHandler.GetByID)
			hypervisors.PUT("/:id", middleware.RequirePermission("hypervisor", "write"), hypervisorHandler.Update)
			hypervisors.DELETE("/:id", middleware.RequirePermission("hypervisor", "delete"), hypervisorHandler.Delete)
			hypervisors.POST("/:id/test-connection",
				middleware.RequirePermission("hypervisor", "read"),
				middleware.ProviderRateLimit(10),
				middleware.ProviderCircuitBreaker(),
				hypervisorHandler.TestConnection)
			hypervisors.POST("/:id/test",
				middleware.RequirePermission("hypervisor", "read"),
				middleware.ProviderRateLimit(10),
				middleware.ProviderCircuitBreaker(),
				hypervisorHandler.TestConnection) // alias
			hypervisors.POST("/:id/sync",
				middleware.RequirePermission("hypervisor", "write"),
				middleware.ProviderRateLimit(5),
				middleware.ProviderCircuitBreaker(),
				middleware.DeduplicateSync(),
				hypervisorHandler.SyncInventory)
		}

		// VMs
		vms := v1.Group("/vms")
		{
			vms.GET("", middleware.RequirePermission("vm", "read"), vmHandler.List)
			vms.GET("/:id", middleware.RequirePermission("vm", "read"), vmHandler.GetByID)
			vms.DELETE("/:id", middleware.RequirePermission("vm", "delete"), vmHandler.Delete)
			vms.POST("/:id/power-on", middleware.RequirePermission("vm", "write"), vmHandler.PowerOn)
			vms.POST("/:id/power-off", middleware.RequirePermission("vm", "write"), vmHandler.PowerOff)
			vms.POST("/:id/reboot", middleware.RequirePermission("vm", "write"), vmHandler.Reboot)
			vms.POST("/:id/suspend", middleware.RequirePermission("vm", "write"), vmHandler.Suspend)
			vms.GET("/:id/metrics", middleware.RequirePermission("vm", "read"), vmHandler.GetMetrics)
			vms.POST("/:id/console", middleware.RequirePermission("vm", "read"), consoleHandler.RequestSession)
			vms.GET("/:id/snapshots", middleware.RequirePermission("vm", "read"), vmHandler.ListSnapshots)
			vms.POST("/:id/snapshots", middleware.RequirePermission("vm", "write"), vmHandler.CreateSnapshot)
			vms.DELETE("/:id/snapshots/:snapshotId", middleware.RequirePermission("vm", "delete"), vmHandler.DeleteSnapshot)
			vms.POST("/:id/snapshots/:snapshotId/revert", middleware.RequirePermission("vm", "write"), vmHandler.RevertSnapshot)
			vms.GET("/:id/tasks", middleware.RequirePermission("task", "read"), vmHandler.ListTasks)
			vms.GET("/:id/activity", middleware.RequirePermission("audit", "read"), vmHandler.ListActivity)
			// Tags on a VM
			vms.GET("/:id/tags", middleware.RequirePermission("vm", "read"), tagHandler.ListByVM)
			vms.POST("/:id/tags", middleware.RequirePermission("vm", "write"), tagHandler.AddToVM)
			vms.DELETE("/:id/tags/:tagId", middleware.RequirePermission("vm", "write"), tagHandler.RemoveFromVM)
			// Bulk operations — must be registered BEFORE /:id to avoid route conflicts
			vms.POST("/bulk/poweron", middleware.RequirePermission("vm", "write"), vmHandler.BulkPowerOn)
			vms.POST("/bulk/poweroff", middleware.RequirePermission("vm", "write"), vmHandler.BulkPowerOff)
			vms.POST("/bulk/reboot", middleware.RequirePermission("vm", "write"), vmHandler.BulkReboot)
			vms.POST("/bulk/snapshot", middleware.RequirePermission("vm", "write"), vmHandler.BulkSnapshot)
			// Clone & provision — registered before /:id
			vms.POST("/clone", middleware.RequirePermission("vm", "write"), provisioningHandler.CloneVM)
			vms.POST("/provision", middleware.RequirePermission("vm", "write"), provisioningHandler.ProvisionVM)
		}

		// Tags (global CRUD)
		tags := v1.Group("/tags")
		{
			tags.GET("", middleware.RequirePermission("vm", "read"), tagHandler.List)
			tags.POST("", middleware.RequirePermission("vm", "write"), tagHandler.Create)
			tags.GET("/:id", middleware.RequirePermission("vm", "read"), tagHandler.GetByID)
			tags.PUT("/:id", middleware.RequirePermission("vm", "write"), tagHandler.Update)
			tags.DELETE("/:id", middleware.RequirePermission("vm", "delete"), tagHandler.Delete)
		}

		// Tasks
		tasks := v1.Group("/tasks")
		{
			tasks.GET("", middleware.RequirePermission("task", "read"), taskHandler.List)
			tasks.GET("/:id", middleware.RequirePermission("task", "read"), taskHandler.GetByID)
			tasks.DELETE("/:id", middleware.RequirePermission("task", "write"), taskHandler.Cancel)
			tasks.GET("/:id/logs", middleware.RequirePermission("task", "read"), taskHandler.GetLogs)
		}

		// Audit
		audit := v1.Group("/audit")
		{
			audit.GET("", middleware.RequirePermission("audit", "read"), auditHandler.List)
			audit.GET("/export", middleware.RequirePermission("audit", "read"), auditHandler.Export)
		}

		// Provider health
		providers := v1.Group("/providers")
		{
			providers.GET("/health", middleware.RequirePermission("hypervisor", "read"), providerHealthHandler.ListAll)
			providers.GET("/:id/health", middleware.RequirePermission("hypervisor", "read"), providerHealthHandler.GetByHypervisor)
			providers.GET("/:id/health/history", middleware.RequirePermission("hypervisor", "read"), providerHealthHandler.GetHistory)
			providers.POST("/:id/health/check", middleware.RequirePermission("hypervisor", "write"), providerHealthHandler.TriggerCheck)
		}

		// Console sessions — lookup by opaque token + WebSocket proxy
		// Note: the WS proxy uses WSAuth so the JWT can be passed as ?token= query param
		// (browsers cannot set headers on WebSocket upgrade requests).
		v1.GET("/consoles/:token", middleware.RequirePermission("vm", "read"), consoleHandler.GetSession)

		// Platform Events
		events := v1.Group("/events")
		{
			events.GET("", middleware.RequirePermission("audit", "read"), eventHandler.List)
			events.GET("/:id", middleware.RequirePermission("audit", "read"), eventHandler.GetByID)
		}

		// Notification Channels
		notifChannels := v1.Group("/notification-channels")
		{
			notifChannels.GET("", middleware.RequirePermission("audit", "read"), notificationHandler.ListChannels)
			notifChannels.POST("", middleware.RequirePermission("audit", "read"), notificationHandler.CreateChannel)
			notifChannels.GET("/:id", middleware.RequirePermission("audit", "read"), notificationHandler.GetChannel)
			notifChannels.PUT("/:id", middleware.RequirePermission("audit", "read"), notificationHandler.UpdateChannel)
			notifChannels.DELETE("/:id", middleware.RequirePermission("audit", "read"), notificationHandler.DeleteChannel)
			notifChannels.POST("/:id/test", middleware.RequirePermission("audit", "read"), notificationHandler.TestChannel)
		}

		// Notification Rules
		notifRules := v1.Group("/notification-rules")
		{
			notifRules.GET("", middleware.RequirePermission("audit", "read"), notificationHandler.ListRules)
			notifRules.POST("", middleware.RequirePermission("audit", "read"), notificationHandler.CreateRule)
			notifRules.GET("/:id", middleware.RequirePermission("audit", "read"), notificationHandler.GetRule)
			notifRules.PUT("/:id", middleware.RequirePermission("audit", "read"), notificationHandler.UpdateRule)
			notifRules.DELETE("/:id", middleware.RequirePermission("audit", "read"), notificationHandler.DeleteRule)
		}

		// Notification History
		v1.GET("/notification-history", middleware.RequirePermission("audit", "read"), notificationHandler.ListHistory)

		// Templates
		templates := v1.Group("/templates")
		{
			templates.GET("",     middleware.RequirePermission("vm", "read"),  templateHandler.List)
			templates.GET("/:id", middleware.RequirePermission("vm", "read"),  templateHandler.GetByID)
		}

		// Provisioning jobs
		provJobs := v1.Group("/provisioning-jobs")
		{
			provJobs.GET("",     middleware.RequirePermission("vm", "read"),  provisioningHandler.ListJobs)
			provJobs.GET("/:id", middleware.RequirePermission("vm", "read"),  provisioningHandler.GetJob)
		}

		// Environments & Stack Orchestration
		environments := v1.Group("/environments")
		{
			environments.GET("",    middleware.RequirePermission("vm", "read"),   environmentHandler.List)
			environments.POST("",   middleware.RequirePermission("vm", "write"),  environmentHandler.Create)
			environments.GET("/:id",  middleware.RequirePermission("vm", "read"),   environmentHandler.GetByID)
			environments.PUT("/:id",  middleware.RequirePermission("vm", "write"),  environmentHandler.Update)
			environments.DELETE("/:id", middleware.RequirePermission("vm", "delete"), environmentHandler.Delete)

			// VM membership
			environments.GET("/:id/vms",           middleware.RequirePermission("vm", "read"),   environmentHandler.ListVMs)
			environments.POST("/:id/vms",          middleware.RequirePermission("vm", "write"),  environmentHandler.AddVM)
			environments.PUT("/:id/vms/:vmId",     middleware.RequirePermission("vm", "write"),  environmentHandler.UpdateVMOrdering)
			environments.DELETE("/:id/vms/:vmId",  middleware.RequirePermission("vm", "write"),  environmentHandler.RemoveVM)

			// Dependencies
			environments.GET("/:id/dependencies",          middleware.RequirePermission("vm", "read"),   environmentHandler.ListDependencies)
			environments.POST("/:id/dependencies",         middleware.RequirePermission("vm", "write"),  environmentHandler.AddDependency)
			environments.DELETE("/:id/dependencies/:depId", middleware.RequirePermission("vm", "write"), environmentHandler.RemoveDependency)

			// Orchestration operations
			environments.POST("/:id/start",    middleware.RequirePermission("vm", "write"), environmentHandler.Start)
			environments.POST("/:id/stop",     middleware.RequirePermission("vm", "write"), environmentHandler.Stop)
			environments.POST("/:id/restart",  middleware.RequirePermission("vm", "write"), environmentHandler.Restart)
			environments.POST("/:id/snapshot", middleware.RequirePermission("vm", "write"), environmentHandler.Snapshot)
			environments.POST("/:id/clone",    middleware.RequirePermission("vm", "write"), environmentHandler.Clone)

			// Status refresh
			environments.POST("/:id/refresh-status", middleware.RequirePermission("vm", "read"), environmentHandler.RefreshStatus)

			// Run tracking
			environments.GET("/:id/runs",              middleware.RequirePermission("vm", "read"), environmentHandler.ListRuns)
			environments.GET("/:id/runs/:runId",       middleware.RequirePermission("vm", "read"), environmentHandler.GetRun)
			environments.GET("/:id/runs/:runId/steps", middleware.RequirePermission("vm", "read"), environmentHandler.GetRunSteps)
		}
	}

	// Template sync per hypervisor (inside the auth-protected v1 group scope)
	v1.POST("/hypervisors/:id/templates/sync",
		middleware.RequirePermission("hypervisor", "write"), templateHandler.SyncTemplates)

	// ── Schedules ─────────────────────────────────────────────────────────────
	schedules := v1.Group("/schedules")
	{
		schedules.GET("",          middleware.RequirePermission("schedule", "read"),  scheduleHandler.List)
		schedules.POST("",         middleware.RequirePermission("schedule", "write"), scheduleHandler.Create)
		schedules.GET("/:id",      middleware.RequirePermission("schedule", "read"),  scheduleHandler.GetByID)
		schedules.PUT("/:id",      middleware.RequirePermission("schedule", "write"), scheduleHandler.Update)
		schedules.DELETE("/:id",   middleware.RequirePermission("schedule", "delete"),scheduleHandler.Delete)
		schedules.POST("/:id/enable",   middleware.RequirePermission("schedule", "write"), scheduleHandler.Enable)
		schedules.POST("/:id/disable",  middleware.RequirePermission("schedule", "write"), scheduleHandler.Disable)
		schedules.POST("/:id/trigger",  middleware.RequirePermission("schedule", "write"), scheduleHandler.TriggerNow)
		schedules.GET("/:id/executions",middleware.RequirePermission("schedule", "read"),  scheduleHandler.ListExecutions)
	}

	// ── Automation Workflows ──────────────────────────────────────────────────
	workflows := v1.Group("/workflows")
	{
		workflows.GET("",              middleware.RequirePermission("workflow", "read"),  workflowHandler.List)
		workflows.POST("",             middleware.RequirePermission("workflow", "write"), workflowHandler.Create)
		workflows.GET("/:id",          middleware.RequirePermission("workflow", "read"),  workflowHandler.GetByID)
		workflows.PUT("/:id",          middleware.RequirePermission("workflow", "write"), workflowHandler.Update)
		workflows.DELETE("/:id",       middleware.RequirePermission("workflow", "delete"),workflowHandler.Delete)
		workflows.POST("/:id/enable",  middleware.RequirePermission("workflow", "write"), workflowHandler.Enable)
		workflows.POST("/:id/disable", middleware.RequirePermission("workflow", "write"), workflowHandler.Disable)
		workflows.POST("/:id/trigger", middleware.RequirePermission("workflow", "write"), workflowHandler.TriggerNow)
		workflows.GET("/:id/runs",     middleware.RequirePermission("workflow", "read"),  workflowHandler.ListRuns)
		workflows.GET("/:id/runs/:runId", middleware.RequirePermission("workflow", "read"), workflowHandler.GetRun)
	}

	// ── Analytics ─────────────────────────────────────────────────────────────
	analyticsGroup := v1.Group("/analytics")
	{
		analyticsGroup.GET("/capacity",                    middleware.RequirePermission("hypervisor", "read"), analyticsHandler.GetCapacity)
		analyticsGroup.GET("/capacity/trends",             middleware.RequirePermission("hypervisor", "read"), analyticsHandler.GetCapacityTrends)
		analyticsGroup.GET("/capacity/providers",          middleware.RequirePermission("hypervisor", "read"), analyticsHandler.GetProviderCapacity)
		analyticsGroup.GET("/recommendations",             middleware.RequirePermission("hypervisor", "read"), analyticsHandler.GetRecommendations)
		analyticsGroup.GET("/recommendations/summary",     middleware.RequirePermission("hypervisor", "read"), analyticsHandler.GetRecommendationSummary)
		analyticsGroup.POST("/recommendations/:id/dismiss",middleware.RequirePermission("hypervisor", "write"), analyticsHandler.DismissRecommendation)
		analyticsGroup.POST("/recommendations/:id/resolve",middleware.RequirePermission("hypervisor", "write"), analyticsHandler.ResolveRecommendation)
		analyticsGroup.GET("/forecasting",                 middleware.RequirePermission("hypervisor", "read"), analyticsHandler.GetForecasts)
		analyticsGroup.POST("/collect",                    middleware.RequirePermission("hypervisor", "write"), analyticsHandler.TriggerCollection)
	}

	// ── Policies ──────────────────────────────────────────────────────────────
	policies := v1.Group("/policies")
	{
		policies.GET("",                                    middleware.RequirePermission("policy", "read"),   policyHandler.List)
		policies.POST("",                                   middleware.RequirePermission("policy", "write"),  policyHandler.Create)
		policies.GET("/:id",                                middleware.RequirePermission("policy", "read"),   policyHandler.GetByID)
		policies.PUT("/:id",                                middleware.RequirePermission("policy", "write"),  policyHandler.Update)
		policies.DELETE("/:id",                             middleware.RequirePermission("policy", "delete"), policyHandler.Delete)
		policies.POST("/:id/enable",                        middleware.RequirePermission("policy", "write"),  policyHandler.Enable)
		policies.POST("/:id/disable",                       middleware.RequirePermission("policy", "write"),  policyHandler.Disable)
		policies.GET("/:id/assignments",                    middleware.RequirePermission("policy", "read"),   policyHandler.ListAssignments)
		policies.POST("/:id/assignments",                   middleware.RequirePermission("policy", "write"),  policyHandler.Assign)
		policies.DELETE("/:id/assignments/:assignmentId",   middleware.RequirePermission("policy", "write"),  policyHandler.Unassign)
	}
	// Policy violations (global list)
	v1.GET("/policy-violations", middleware.RequirePermission("policy", "read"), policyHandler.ListViolations)
	// Policy evaluation endpoint (for testing / pre-flight checks)
	v1.POST("/policies/evaluate", middleware.RequirePermission("policy", "read"), policyHandler.Evaluate)

	// ── Approvals ─────────────────────────────────────────────────────────────
	approvals := v1.Group("/approvals")
	{
		approvals.GET("",                    middleware.RequirePermission("approval", "read"),   approvalHandler.List)
		approvals.GET("/pending",            middleware.RequirePermission("approval", "read"),   approvalHandler.GetPendingForMe)
		approvals.GET("/:id",                middleware.RequirePermission("approval", "read"),   approvalHandler.GetByID)
		approvals.POST("/:id/approve",       middleware.RequirePermission("approval", "write"),  approvalHandler.Approve)
		approvals.POST("/:id/reject",        middleware.RequirePermission("approval", "write"),  approvalHandler.Reject)
		approvals.POST("/:id/cancel",        middleware.RequirePermission("approval", "write"),  approvalHandler.Cancel)
		approvals.POST("/:id/escalate",      middleware.RequirePermission("approval", "write"),  approvalHandler.Escalate)
	}

	// ── System Health Dashboard ───────────────────────────────────────────────
	system := v1.Group("/system")
	{
		system.GET("/health", middleware.RequirePermission("hypervisor", "read"), systemHealthHandler.GetHealth)
		// Maintenance mode — admin only
		system.GET("/maintenance", middleware.RequirePermission("hypervisor", "read"), maintenanceHandler.GetStatus)
		system.POST("/maintenance/enable", middleware.RequirePermission("hypervisor", "write"), maintenanceHandler.Enable)
		system.POST("/maintenance/disable", middleware.RequirePermission("hypervisor", "write"), maintenanceHandler.Disable)
	}

	// ── Infrastructure Hierarchy ──────────────────────────────────────────────
	infra := v1.Group("/infrastructure")
	{
		infra.GET("/tree", middleware.RequirePermission("hypervisor", "read"), infraHandler.GetTree)
	}
	v1.GET("/hosts",        middleware.RequirePermission("hypervisor", "read"), infraHandler.ListHosts)
	v1.GET("/hosts/:id",    middleware.RequirePermission("hypervisor", "read"), infraHandler.GetHost)
	v1.GET("/clusters",     middleware.RequirePermission("hypervisor", "read"), infraHandler.ListClusters)
	v1.GET("/clusters/:id", middleware.RequirePermission("hypervisor", "read"), infraHandler.GetCluster)
	v1.GET("/datastores",   middleware.RequirePermission("hypervisor", "read"), infraHandler.ListDataStores)
	v1.GET("/networks",     middleware.RequirePermission("hypervisor", "read"), infraHandler.ListNetworks)

	// Console WebSocket proxy — outside the auth middleware group so it can use WSAuth
	// which accepts JWT via ?token= query param (required for WebSocket upgrades).
	r.GET("/api/v1/consoles/:token/ws", middleware.WSAuth(deps.Services.Auth), consoleProxyHandler.ProxyConsole)

	// ── WebSocket ─────────────────────────────────────────────────────────────
	// Browsers cannot set headers on WebSocket connections, so we accept the
	// JWT via the ?token= query parameter as well as the Authorization header.
	r.GET("/ws", middleware.WSAuth(deps.Services.Auth), wsHandler.Handle)

	return r
}
