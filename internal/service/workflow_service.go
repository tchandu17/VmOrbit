package service

import (
	"context"
	"fmt"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/scheduler"
	"github.com/vmOrbit/backend/pkg/logger"
)

type workflowService struct {
	repo     port.WorkflowRepository
	runRepo  port.WorkflowRunRepository
	wfEngine *scheduler.WorkflowEngine
	log      logger.Logger
}

// NewWorkflowService creates a new workflow service.
func NewWorkflowService(
	repo port.WorkflowRepository,
	runRepo port.WorkflowRunRepository,
	wfEngine *scheduler.WorkflowEngine,
	log logger.Logger,
) *workflowService {
	return &workflowService{
		repo:     repo,
		runRepo:  runRepo,
		wfEngine: wfEngine,
		log:      log,
	}
}

func (s *workflowService) Create(ctx context.Context, req port.CreateWorkflowRequest) (*model.Workflow, error) {
	maxConcurrent := req.MaxConcurrentRuns
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	w := &model.Workflow{
		Name:              req.Name,
		Description:       req.Description,
		Enabled:           req.Enabled,
		Status:            model.WorkflowStatusActive,
		TriggerType:       req.TriggerType,
		TriggerConfig:     req.TriggerConfig,
		Conditions:        req.Conditions,
		ContinueOnError:   req.ContinueOnError,
		MaxConcurrentRuns: maxConcurrent,
		CreatedBy:         callerUUID(ctx),
	}
	if !req.Enabled {
		w.Status = model.WorkflowStatusPaused
	}

	for i, a := range req.Actions {
		action := model.WorkflowAction{
			Order:           a.Order,
			ActionType:      a.ActionType,
			Name:            a.Name,
			Description:     a.Description,
			Config:          a.Config,
			RetryCount:      a.RetryCount,
			TimeoutSeconds:  a.TimeoutSeconds,
			ContinueOnError: a.ContinueOnError,
		}
		if action.Order == 0 {
			action.Order = i + 1
		}
		if action.TimeoutSeconds <= 0 {
			action.TimeoutSeconds = 300
		}
		w.Actions = append(w.Actions, action)
	}

	if err := s.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("creating workflow: %w", err)
	}
	return w, nil
}

func (s *workflowService) GetByID(ctx context.Context, id string) (*model.Workflow, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *workflowService) Update(ctx context.Context, id string, req port.UpdateWorkflowRequest) (*model.Workflow, error) {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		w.Name = *req.Name
	}
	if req.Description != nil {
		w.Description = *req.Description
	}
	if req.Enabled != nil {
		w.Enabled = *req.Enabled
		if *req.Enabled {
			w.Status = model.WorkflowStatusActive
		} else {
			w.Status = model.WorkflowStatusPaused
		}
	}
	if req.TriggerType != nil {
		w.TriggerType = *req.TriggerType
	}
	if req.TriggerConfig != nil {
		w.TriggerConfig = req.TriggerConfig
	}
	if req.Conditions != nil {
		w.Conditions = req.Conditions
	}
	if req.ContinueOnError != nil {
		w.ContinueOnError = *req.ContinueOnError
	}
	if req.MaxConcurrentRuns != nil {
		w.MaxConcurrentRuns = *req.MaxConcurrentRuns
	}

	if req.Actions != nil {
		w.Actions = nil
		for i, a := range req.Actions {
			action := model.WorkflowAction{
				WorkflowID:      w.ID,
				Order:           a.Order,
				ActionType:      a.ActionType,
				Name:            a.Name,
				Description:     a.Description,
				Config:          a.Config,
				RetryCount:      a.RetryCount,
				TimeoutSeconds:  a.TimeoutSeconds,
				ContinueOnError: a.ContinueOnError,
			}
			if action.Order == 0 {
				action.Order = i + 1
			}
			if action.TimeoutSeconds <= 0 {
				action.TimeoutSeconds = 300
			}
			w.Actions = append(w.Actions, action)
		}
	}

	if err := s.repo.Update(ctx, w); err != nil {
		return nil, fmt.Errorf("updating workflow: %w", err)
	}
	return w, nil
}

func (s *workflowService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *workflowService) List(ctx context.Context, filter port.WorkflowFilter, page port.Page) (*port.PageResult[model.Workflow], error) {
	return s.repo.List(ctx, filter, page)
}

func (s *workflowService) Enable(ctx context.Context, id string) error {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	w.Enabled = true
	w.Status = model.WorkflowStatusActive
	return s.repo.Update(ctx, w)
}

func (s *workflowService) Disable(ctx context.Context, id string) error {
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	w.Enabled = false
	w.Status = model.WorkflowStatusPaused
	return s.repo.Update(ctx, w)
}

func (s *workflowService) TriggerNow(ctx context.Context, id string, triggerData model.JSONMap) (string, error) {
	runID, err := s.wfEngine.TriggerManual(ctx, id, triggerData, callerUUID(ctx))
	if err != nil {
		return "", fmt.Errorf("trigger workflow: %w", err)
	}
	return runID, nil
}

func (s *workflowService) ListRuns(ctx context.Context, workflowID string, page port.Page) (*port.PageResult[model.WorkflowRun], error) {
	return s.runRepo.List(ctx, workflowID, page)
}

func (s *workflowService) GetRun(ctx context.Context, runID string) (*model.WorkflowRun, error) {
	return s.runRepo.GetByID(ctx, runID)
}
