package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

type provisioningService struct {
	jobs        port.ProvisioningJobRepository
	vms         port.VMRepository
	templates   port.VMTemplateRepository
	hypervisors port.HypervisorRepository
	tasks       port.TaskRepository
	audit       port.AuditService
	log         logger.Logger
}

// NewProvisioningService creates a new provisioning service.
func NewProvisioningService(
	jobs port.ProvisioningJobRepository,
	vms port.VMRepository,
	templates port.VMTemplateRepository,
	hypervisors port.HypervisorRepository,
	tasks port.TaskRepository,
	audit port.AuditService,
	log logger.Logger,
) port.ProvisioningService {
	return &provisioningService{
		jobs:        jobs,
		vms:         vms,
		templates:   templates,
		hypervisors: hypervisors,
		tasks:       tasks,
		audit:       audit,
		log:         log,
	}
}

// Clone creates a clone of an existing VM.
func (s *provisioningService) Clone(ctx context.Context, req port.CloneVMRequest) (*model.ProvisioningJob, error) {
	// Validate source VM exists.
	vm, err := s.vms.GetByID(ctx, req.SourceVMID)
	if err != nil {
		return nil, fmt.Errorf("source VM not found: %w", err)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("clone name is required")
	}

	vmUUID := vm.ID
	hypervisorUUID := vm.HypervisorID

	// Create the provisioning job record.
	job := &model.ProvisioningJob{
		Type:         model.ProvisioningJobTypeClone,
		Status:       model.ProvisioningJobStatusPending,
		SourceVMID:   &vmUUID,
		HypervisorID: hypervisorUUID,
		VMName:       req.Name,
		DataStore:    req.DataStore,
		Node:         req.Node,
		Linked:       req.Linked,
		CreatedBy:    callerUUID(ctx),
	}
	if len(req.Tags) > 0 {
		job.Tags = model.StringArray(req.Tags)
	}
	if len(req.Metadata) > 0 {
		job.Metadata = req.Metadata
	}
	job.ID = uuid.New()

	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("creating provisioning job: %w", err)
	}

	// Create the async task.
	now := time.Now().UTC()
	jobID := job.ID
	t := &model.Task{
		Type:         model.TaskTypeVMCloneOp,
		Status:       model.TaskStatusPending,
		Priority:     4,
		MaxRetries:   2,
		VMID:         &vmUUID,
		HypervisorID: &hypervisorUUID,
		ScheduledAt:  &now,
		CreatedBy:    callerUUID(ctx),
		Payload: model.JSONMap{
			"provisioning_job_id": jobID.String(),
			"source_vm_id":        req.SourceVMID,
			"provider_vm_id":      vm.ProviderVMID,
			"hypervisor_id":       vm.HypervisorID.String(),
			"name":                req.Name,
			"data_store":          req.DataStore,
			"node":                req.Node,
			"linked":              req.Linked,
		},
	}
	t.ID = uuid.New()

	if err := s.tasks.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("creating clone task: %w", err)
	}

	// Link task to job.
	taskID := t.ID
	job.TaskID = &taskID
	if err := s.jobs.Update(ctx, job); err != nil {
		s.log.Warn("failed to link task to provisioning job",
			logger.String("job_id", job.ID.String()),
			logger.String("task_id", t.ID.String()),
			logger.Error(err),
		)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionCreate,
		Resource:    "provisioning_job",
		ResourceID:  job.ID.String(),
		Description: fmt.Sprintf("clone VM %q → %q", vm.Name, req.Name),
		Success:     true,
	})

	return job, nil
}

// Provision creates a new VM from a template.
func (s *provisioningService) Provision(ctx context.Context, req port.ProvisionVMRequest) (*model.ProvisioningJob, error) {
	// Validate template exists.
	tmpl, err := s.templates.GetByID(ctx, req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("VM name is required")
	}

	// Apply template defaults for unset fields.
	cpuCount := req.CPUCount
	if cpuCount <= 0 {
		cpuCount = tmpl.CPUCount
	}
	memoryMB := req.MemoryMB
	if memoryMB <= 0 {
		memoryMB = tmpl.MemoryMB
	}
	diskGB := req.DiskGB
	if diskGB <= 0 {
		diskGB = tmpl.DiskGB
	}

	templateID := tmpl.ID
	hypervisorUUID := tmpl.HypervisorID

	// Create the provisioning job record.
	job := &model.ProvisioningJob{
		Type:         model.ProvisioningJobTypeProvision,
		Status:       model.ProvisioningJobStatusPending,
		TemplateID:   &templateID,
		HypervisorID: hypervisorUUID,
		VMName:       req.Name,
		CPUCount:     cpuCount,
		MemoryMB:     memoryMB,
		DiskGB:       diskGB,
		NetworkName:  req.NetworkName,
		DataStore:    req.DataStore,
		Node:         req.Node,
		CreatedBy:    callerUUID(ctx),
	}
	if len(req.Tags) > 0 {
		job.Tags = model.StringArray(req.Tags)
	}
	if len(req.Metadata) > 0 {
		job.Metadata = req.Metadata
	}
	job.ID = uuid.New()

	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("creating provisioning job: %w", err)
	}

	// Create the async task.
	now := time.Now().UTC()
	jobID := job.ID
	t := &model.Task{
		Type:         model.TaskTypeVMProvision,
		Status:       model.TaskStatusPending,
		Priority:     4,
		MaxRetries:   2,
		HypervisorID: &hypervisorUUID,
		ScheduledAt:  &now,
		CreatedBy:    callerUUID(ctx),
		Payload: model.JSONMap{
			"provisioning_job_id": jobID.String(),
			"template_id":         req.TemplateID,
			"provider_template_id": tmpl.ProviderID,
			"hypervisor_id":       tmpl.HypervisorID.String(),
			"name":                req.Name,
			"cpu_count":           cpuCount,
			"memory_mb":           memoryMB,
			"disk_gb":             diskGB,
			"network_name":        req.NetworkName,
			"data_store":          req.DataStore,
			"node":                req.Node,
		},
	}
	t.ID = uuid.New()

	if err := s.tasks.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("creating provision task: %w", err)
	}

	// Link task to job.
	taskID := t.ID
	job.TaskID = &taskID
	if err := s.jobs.Update(ctx, job); err != nil {
		s.log.Warn("failed to link task to provisioning job",
			logger.String("job_id", job.ID.String()),
			logger.String("task_id", t.ID.String()),
			logger.Error(err),
		)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionCreate,
		Resource:    "provisioning_job",
		ResourceID:  job.ID.String(),
		Description: fmt.Sprintf("provision VM %q from template %q", req.Name, tmpl.Name),
		Success:     true,
	})

	return job, nil
}

func (s *provisioningService) GetJob(ctx context.Context, id string) (*model.ProvisioningJob, error) {
	return s.jobs.GetByID(ctx, id)
}

func (s *provisioningService) ListJobs(ctx context.Context, filter port.ProvisioningJobFilter, page port.Page) (*port.PageResult[model.ProvisioningJob], error) {
	return s.jobs.List(ctx, filter, page)
}
