package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

type tagService struct {
	tags port.TagRepository
	vms  port.VMRepository
	log  logger.Logger
}

// NewTagService creates a new TagService.
func NewTagService(tags port.TagRepository, vms port.VMRepository, log logger.Logger) port.TagService {
	return &tagService{tags: tags, vms: vms, log: log}
}

func (s *tagService) Create(ctx context.Context, req port.CreateTagRequest) (*model.Tag, error) {
	// Default color if not provided.
	color := req.Color
	if color == "" {
		color = "#6B7280"
	}

	tag := &model.Tag{
		Base:        model.Base{ID: uuid.New()},
		Name:        req.Name,
		Color:       color,
		Description: req.Description,
	}

	if err := s.tags.Create(ctx, tag); err != nil {
		return nil, fmt.Errorf("creating tag: %w", err)
	}
	return tag, nil
}

func (s *tagService) GetByID(ctx context.Context, id string) (*model.Tag, error) {
	return s.tags.GetByID(ctx, id)
}

func (s *tagService) Update(ctx context.Context, id string, req port.UpdateTagRequest) (*model.Tag, error) {
	tag, err := s.tags.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		tag.Name = *req.Name
	}
	if req.Color != nil {
		tag.Color = *req.Color
	}
	if req.Description != nil {
		tag.Description = *req.Description
	}
	if err := s.tags.Update(ctx, tag); err != nil {
		return nil, fmt.Errorf("updating tag: %w", err)
	}
	return tag, nil
}

func (s *tagService) Delete(ctx context.Context, id string) error {
	return s.tags.Delete(ctx, id)
}

func (s *tagService) List(ctx context.Context) ([]model.Tag, error) {
	return s.tags.List(ctx)
}

func (s *tagService) AddToVM(ctx context.Context, vmID, tagID string) error {
	// Verify both exist.
	if _, err := s.vms.GetByID(ctx, vmID); err != nil {
		return fmt.Errorf("vm not found: %w", err)
	}
	if _, err := s.tags.GetByID(ctx, tagID); err != nil {
		return fmt.Errorf("tag not found: %w", err)
	}
	return s.tags.AddToVM(ctx, vmID, tagID)
}

func (s *tagService) RemoveFromVM(ctx context.Context, vmID, tagID string) error {
	return s.tags.RemoveFromVM(ctx, vmID, tagID)
}

func (s *tagService) ListByVM(ctx context.Context, vmID string) ([]model.Tag, error) {
	return s.tags.ListByVM(ctx, vmID)
}

func (s *tagService) ListVMsByTag(ctx context.Context, tagID string) ([]string, error) {
	return s.tags.ListVMsByTag(ctx, tagID)
}
