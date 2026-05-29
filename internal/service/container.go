package service

import (
	"gorm.io/gorm"

	"github.com/redis/go-redis/v9"
	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/infrastructure/cache"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/internal/infrastructure/repository"
	"github.com/vmOrbit/backend/internal/notification"
	"github.com/vmOrbit/backend/internal/policy"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/internal/scheduler"
	"github.com/vmOrbit/backend/pkg/logger"
)

// Repositories groups all repository interfaces.
type Repositories struct {
	Users                 port.UserRepository
	Roles                 port.RoleRepository
	Permissions           port.PermissionRepository
	Hypervisors           port.HypervisorRepository
	HypervisorGroups      port.HypervisorGroupRepository
	VMs                   port.VMRepository
	VMTemplates           port.VMTemplateRepository
	Snapshots             port.SnapshotRepository
	DataStores            port.DataStoreRepository
	Networks              port.NetworkRepository
	Tasks                 port.TaskRepository
	Audit                 port.AuditRepository
	Events                port.WebSocketEventRepository
	ConsoleSessions       port.ConsoleSessionRepository
	Tags                  port.TagRepository
	ProviderHealth        port.ProviderHealthRepository
	PlatformEvents        port.PlatformEventRepository
	NotificationChannels  port.NotificationChannelRepository
	NotificationRules     port.NotificationRuleRepository
	NotificationHistory   port.NotificationHistoryRepository
	Schedules             port.ScheduleRepository
	ScheduleExecutions    port.ScheduleExecutionRepository
	Workflows             port.WorkflowRepository
	WorkflowRuns          port.WorkflowRunRepository
	ProvisioningJobs      port.ProvisioningJobRepository
	// Environment & orchestration
	Environments          port.EnvironmentRepository
	EnvironmentVMs        port.EnvironmentVMRepository
	VMDependencies        port.VMDependencyRepository
	OrchestrationRuns     port.OrchestrationRunRepository
	OrchestrationSteps    port.OrchestrationStepRepository
	// Analytics
	InfraMetrics          port.InfrastructureMetricsRepository
	CapacityHistory       port.CapacityHistoryRepository
	ProviderCapacity      port.ProviderCapacityRepository
	VMUsageStats          port.VMUsageStatsRepository
	Recommendations       port.OptimizationRecommendationRepository
	RecommendationHistory port.RecommendationHistoryRepository
	// Policy & Approval governance
	Policies          port.PolicyRepository
	PolicyAssignments port.PolicyAssignmentRepository
	PolicyViolations  port.PolicyViolationRepository
	ApprovalRequests  port.ApprovalRequestRepository
	ApprovalSteps     port.ApprovalStepRepository
	ApprovalHistory   port.ApprovalHistoryRepository
	// Infrastructure hierarchy
	Clusters port.ClusterRepository
	Hosts    port.HostRepository
}

// NewRepositories wires GORM-backed repositories.
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Users:                repository.NewUserRepo(db),
		Roles:                repository.NewRoleRepo(db),
		Permissions:          repository.NewPermissionRepo(db),
		Hypervisors:          repository.NewHypervisorRepo(db),
		HypervisorGroups:     repository.NewHypervisorGroupRepo(db),
		VMs:                  repository.NewVMRepo(db),
		VMTemplates:          repository.NewTemplateRepo(db),
		Snapshots:            repository.NewSnapshotRepo(db),
		DataStores:           repository.NewDataStoreRepo(db),
		Networks:             repository.NewNetworkRepo(db),
		Tasks:                repository.NewTaskRepo(db),
		Audit:                repository.NewAuditRepo(db),
		Events:               repository.NewEventRepo(db),
		ConsoleSessions:      repository.NewConsoleRepo(db),
		Tags:                 repository.NewTagRepo(db),
		ProviderHealth:       repository.NewProviderHealthRepo(db),
		PlatformEvents:       repository.NewPlatformEventRepo(db),
		NotificationChannels: repository.NewNotificationChannelRepo(db),
		NotificationRules:    repository.NewNotificationRuleRepo(db),
		NotificationHistory:  repository.NewNotificationHistoryRepo(db),
		Schedules:            repository.NewScheduleRepo(db),
		ScheduleExecutions:   repository.NewScheduleExecutionRepo(db),
		Workflows:            repository.NewWorkflowRepo(db),
		WorkflowRuns:         repository.NewWorkflowRunRepo(db),
		ProvisioningJobs:     repository.NewProvisioningJobRepo(db),
		// Environment & orchestration
		Environments:       repository.NewEnvironmentRepo(db),
		EnvironmentVMs:     repository.NewEnvironmentVMRepo(db),
		VMDependencies:     repository.NewVMDependencyRepo(db),
		OrchestrationRuns:  repository.NewOrchestrationRunRepo(db),
		OrchestrationSteps: repository.NewOrchestrationStepRepo(db),
		// Analytics
		InfraMetrics:          repository.NewInfrastructureMetricsRepo(db),
		CapacityHistory:       repository.NewCapacityHistoryRepo(db),
		ProviderCapacity:      repository.NewProviderCapacityRepo(db),
		VMUsageStats:          repository.NewVMUsageStatsRepo(db),
		Recommendations:       repository.NewOptimizationRecommendationRepo(db),
		RecommendationHistory: repository.NewRecommendationHistoryRepo(db),
		// Policy & Approval governance
		Policies:          repository.NewPolicyRepo(db),
		PolicyAssignments: repository.NewPolicyAssignmentRepo(db),
		PolicyViolations:  repository.NewPolicyViolationRepo(db),
		ApprovalRequests:  repository.NewApprovalRequestRepo(db),
		ApprovalSteps:     repository.NewApprovalStepRepo(db),
		ApprovalHistory:   repository.NewApprovalHistoryRepo(db),
		// Infrastructure hierarchy
		Clusters: repository.NewClusterRepo(db),
		Hosts:    repository.NewHostRepo(db),
	}
}

// Services groups all service interfaces.
type Services struct {
	Auth                  port.AuthService
	Users                 port.UserService
	Roles                 port.RoleService
	Permissions           port.PermissionService
	Hypervisors           port.HypervisorService
	VMs                   port.VMService
	Tasks                 port.TaskService
	Audit                 port.AuditService
	Console               port.ConsoleService
	Tags                  port.TagService
	ProviderHealth        port.ProviderHealthService
	PlatformEvents        port.PlatformEventService
	NotificationChannels  port.NotificationChannelService
	NotificationRules     port.NotificationRuleService
	NotificationHistory   port.NotificationHistoryService
	Schedules             port.ScheduleService
	Workflows             port.WorkflowService
	Templates             port.TemplateService
	Provisioning          port.ProvisioningService
	// Environment & orchestration
	Environments          port.EnvironmentService
	// Analytics
	Analytics             port.AnalyticsService
	// Policy & Approval governance
	Policies   port.PolicyService
	Approvals  port.ApprovalService
	// Infrastructure hierarchy
	Infrastructure port.InfrastructureService
}

// Deps carries all dependencies needed to build the service layer.
type Deps struct {
	Repos           *Repositories
	Cache           cache.Cache
	RedisClient     *redis.Client
	Registry        *provider.Registry
	ProviderManager *provider.Manager
	EventBus        messaging.EventBus
	Log             logger.Logger
	Config          *config.Config
}

// NewServices wires all service implementations.
func NewServices(d Deps) *Services {
	auditSvc := NewAuditService(d.Repos.Audit, d.Log)

	// Notification dispatcher — evaluates rules and delivers notifications
	notifDispatcher := notification.NewDispatcher(
		d.Repos.NotificationRules,
		d.Repos.NotificationHistory,
		d.Log,
	)

	platformEventSvc := NewPlatformEventService(d.Repos.PlatformEvents, d.EventBus, notifDispatcher, d.Log)

	// Workflow engine — subscribes to event bus and fires automation workflows.
	// The engine is created here but Start() is called from bootstrap after wiring.
	wfEngine := scheduler.NewWorkflowEngine(scheduler.WorkflowEngineDeps{
		Workflows:    d.Repos.Workflows,
		WorkflowRuns: d.Repos.WorkflowRuns,
		EventBus:     d.EventBus,
		Log:          d.Log,
		// Ops and Enqueue are injected after the full service graph is built.
	})

	scheduleSvc := NewScheduleService(d.Repos.Schedules, d.Repos.ScheduleExecutions, d.Log)
	workflowSvc := NewWorkflowService(d.Repos.Workflows, d.Repos.WorkflowRuns, wfEngine, d.Log)

	svcs := &Services{
		Auth:           NewAuthService(d.Repos.Users, d.Repos.Permissions, d.Cache, d.Config.JWT, d.Log),
		Users:          NewUserService(d.Repos.Users, auditSvc, d.Log),
		Roles:          NewRoleService(d.Repos.Roles, d.Repos.Permissions, auditSvc, d.Log),
		Permissions:    NewPermissionService(d.Repos.Permissions, auditSvc, d.Log),
		Hypervisors:    NewHypervisorService(d.Repos.Hypervisors, d.Registry, d.Repos.Tasks, d.Repos.VMs, d.Repos.DataStores, d.Repos.Networks, d.Repos.Clusters, d.Repos.Hosts, d.EventBus, auditSvc, d.Repos.ProviderHealth, d.Log),
		VMs:            NewVMService(d.Repos.VMs, d.Repos.Hypervisors, d.Repos.Tasks, d.Repos.Snapshots, d.Registry, auditSvc, d.Log),
		Tasks:          NewTaskService(d.Repos.Tasks, d.RedisClient, d.Log),
		Audit:          auditSvc,
		Console:        NewConsoleService(d.Repos.ConsoleSessions, d.Repos.VMs, d.Repos.Hypervisors, d.Registry, auditSvc, d.Log),
		Tags:           NewTagService(d.Repos.Tags, d.Repos.VMs, d.Log),
		ProviderHealth: NewProviderHealthService(d.Repos.ProviderHealth, d.Repos.Hypervisors, d.Repos.Audit, d.Registry, d.Log),
		PlatformEvents: platformEventSvc,
		NotificationChannels: NewNotificationChannelService(d.Repos.NotificationChannels, d.Log),
		NotificationRules:    NewNotificationRuleService(d.Repos.NotificationRules, d.Repos.NotificationChannels, d.Log),
		NotificationHistory:  NewNotificationHistoryService(d.Repos.NotificationHistory, d.Log),
		Schedules:            scheduleSvc,
		Workflows:            workflowSvc,
		Templates:            NewTemplateService(d.Repos.VMTemplates, d.Repos.Hypervisors, d.Repos.Tasks, d.Registry, auditSvc, d.Log),
		Provisioning:         NewProvisioningService(d.Repos.ProvisioningJobs, d.Repos.VMs, d.Repos.VMTemplates, d.Repos.Hypervisors, d.Repos.Tasks, auditSvc, d.Log),
		// Environment & orchestration
		Environments: NewEnvironmentService(
			d.Repos.Environments,
			d.Repos.EnvironmentVMs,
			d.Repos.VMDependencies,
			d.Repos.OrchestrationRuns,
			d.Repos.OrchestrationSteps,
			d.Repos.VMs,
			d.Repos.Tasks,
			nil, // vmSvc injected below to avoid circular dependency
			auditSvc,
			d.Log,
		),
	}

	// Inject vmSvc into environment service after both are constructed
	if es, ok := svcs.Environments.(*environmentService); ok {
		es.vmSvc = svcs.VMs
	}

	// Analytics service
	svcs.Analytics = NewAnalyticsService(
		d.Repos.InfraMetrics,
		d.Repos.CapacityHistory,
		d.Repos.ProviderCapacity,
		d.Repos.VMUsageStats,
		d.Repos.Recommendations,
		d.Repos.RecommendationHistory,
		d.Repos.Hypervisors,
		d.Repos.VMs,
		d.Repos.Snapshots,
		d.Repos.DataStores,
		d.Repos.Tasks,
		d.Repos.Environments,
		d.Log,
	)

	// Policy & Approval governance
	// Build approval service first (no circular deps), then policy engine, then policy service.
	approvalSvc := NewApprovalService(
		d.Repos.ApprovalRequests,
		d.Repos.ApprovalSteps,
		d.Repos.ApprovalHistory,
		auditSvc,
		d.Log,
	)
	policyEngine := policy.NewEngine(
		d.Repos.Policies,
		d.Repos.PolicyAssignments,
		d.Repos.PolicyViolations,
		approvalSvc,
		d.Log,
	)
	svcs.Policies = NewPolicyService(
		d.Repos.Policies,
		d.Repos.PolicyAssignments,
		d.Repos.PolicyViolations,
		policyEngine,
		auditSvc,
		d.Log,
	)
	svcs.Approvals = approvalSvc

	// Infrastructure hierarchy service
	svcs.Infrastructure = NewInfrastructureService(
		d.Repos.Clusters,
		d.Repos.Hosts,
		d.Repos.VMs,
		d.Repos.DataStores,
		d.Repos.Networks,
		d.Repos.Hypervisors,
		d.Log,
	)

	return svcs
}

// ExtractWorkflowEngine returns the workflow engine from the workflow service.
func ExtractWorkflowEngine(svc port.WorkflowService) *scheduler.WorkflowEngine {
	if ws, ok := svc.(*workflowService); ok {
		return ws.wfEngine
	}
	return nil
}

// ExtractScheduleService returns the concrete schedule service for engine injection.
func ExtractScheduleService(svc port.ScheduleService) *scheduleService {
	if ss, ok := svc.(*scheduleService); ok {
		return ss
	}
	return nil
}
