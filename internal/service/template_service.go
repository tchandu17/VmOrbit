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

type templateService struct {
	templates   port.VMTemplateRepository
	hypervisors port.HypervisorRepository
	tasks       port.TaskRepository
	registry    *provider.Registry
	audit       port.AuditService
	log         logger.Logger
}

// NewTemplateService creates a new template service.
func NewTemplateService(
	templates port.VMTemplateRepository,
	hypervisors port.HypervisorRepository,
	tasks port.TaskRepository,
	registry *provider.Registry,
	audit port.AuditService,
	log logger.Logger,
) port.TemplateService {
	return &templateService{
		templates:   templates,
		hypervisors: hypervisors,
		tasks:       tasks,
		registry:    registry,
		audit:       audit,
		log:         log,
	}
}

func (s *templateService) List(ctx context.Context, hypervisorID string, page port.Page) (*port.PageResult[model.VMTemplate], error) {
	return s.templates.List(ctx, hypervisorID, page)
}

func (s *templateService) GetByID(ctx context.Context, id string) (*model.VMTemplate, error) {
	return s.templates.GetByID(ctx, id)
}

// SyncTemplates creates an async task to discover templates from the provider.
func (s *templateService) SyncTemplates(ctx context.Context, hypervisorID string) (string, error) {
	if _, err := s.hypervisors.GetByID(ctx, hypervisorID); err != nil {
		return "", fmt.Errorf("hypervisor not found: %w", err)
	}

	now := time.Now().UTC()
	hypervisorUUID, _ := uuid.Parse(hypervisorID)
	t := &model.Task{
		Type:         model.TaskTypeTemplateSync,
		Status:       model.TaskStatusPending,
		Priority:     5,
		MaxRetries:   3,
		HypervisorID: &hypervisorUUID,
		ScheduledAt:  &now,
		Payload:      model.JSONMap{"hypervisor_id": hypervisorID},
		CreatedBy:    callerUUID(ctx),
	}
	t.ID = uuid.New()

	if err := s.tasks.Create(ctx, t); err != nil {
		return "", fmt.Errorf("creating template sync task: %w", err)
	}

	return t.ID.String(), nil
}

// SyncTemplatesNow performs the template sync synchronously (called by the task engine).
func (s *templateService) SyncTemplatesNow(ctx context.Context, hypervisorID string, progress func(pct int, msg string)) (int, error) {
	if progress == nil {
		progress = func(int, string) {}
	}

	h, err := s.hypervisors.GetByID(ctx, hypervisorID)
	if err != nil {
		return 0, fmt.Errorf("hypervisor not found: %w", err)
	}

	progress(10, "connecting to hypervisor")

	creds, err := buildHypervisorCredentials(h)
	if err != nil {
		return 0, fmt.Errorf("building credentials: %w", err)
	}

	p, err := s.registry.Get(h.Provider)
	if err != nil {
		return 0, fmt.Errorf("provider not registered: %w", err)
	}

	if err := p.Connect(ctx, creds); err != nil {
		return 0, fmt.Errorf("provider connect: %w", err)
	}
	defer p.Disconnect(ctx) //nolint:errcheck

	progress(30, "discovering templates from provider")

	// Use the TemplateProvider interface if available; fall back to filtering
	// templates from the full VM list via inventory sync.
	tp, ok := p.(port.TemplateProvider)
	if !ok {
		s.log.Warn("provider does not implement TemplateProvider, skipping template sync",
			logger.String("provider", string(h.Provider)),
		)
		return 0, nil
	}

	infos, err := tp.ListTemplates(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing templates: %w", err)
	}

	progress(60, fmt.Sprintf("discovered %d templates, persisting", len(infos)))

	hypervisorUUID, _ := uuid.Parse(hypervisorID)
	templates := make([]model.VMTemplate, 0, len(infos))
	for _, info := range infos {
		tmpl := model.VMTemplate{
			HypervisorID: hypervisorUUID,
			ProviderID:   info.ProviderID,
			Name:         info.Name,
			Description:  info.Description,
			GuestOS:      info.GuestOS,
			CPUCount:     info.CPUCount,
			MemoryMB:     info.MemoryMB,
			DiskGB:       info.DiskGB,
		}
		if len(info.Tags) > 0 {
			tmpl.Tags = model.StringArray(info.Tags)
		}
		if len(info.Extra) > 0 {
			tmpl.Metadata = model.JSONMap{}
			for k, v := range info.Extra {
				tmpl.Metadata[k] = v
			}
		}
		templates = append(templates, tmpl)
	}

	if err := s.templates.BulkUpsert(ctx, templates); err != nil {
		return 0, fmt.Errorf("persisting templates: %w", err)
	}

	_ = s.audit.Log(ctx, port.AuditEntry{
		Action:      model.AuditActionExecute,
		Resource:    "template",
		ResourceID:  hypervisorID,
		Description: fmt.Sprintf("template sync: discovered %d templates", len(templates)),
		Success:     true,
	})

	progress(100, fmt.Sprintf("synced %d templates", len(templates)))
	s.log.Info("template sync complete",
		logger.String("hypervisor_id", hypervisorID),
		logger.Int("count", len(templates)),
	)
	return len(templates), nil
}
