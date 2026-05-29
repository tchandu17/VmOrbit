package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

type auditService struct {
	repo port.AuditRepository
	log  logger.Logger
}

// NewAuditService creates a new audit service.
func NewAuditService(repo port.AuditRepository, log logger.Logger) port.AuditService {
	return &auditService{repo: repo, log: log}
}

func (s *auditService) Log(ctx context.Context, entry port.AuditEntry) error {
	record := &model.AuditLog{
		ID:           uuid.New(),
		CreatedAt:    time.Now().UTC(),
		Username:     entry.Username,
		Action:       entry.Action,
		Resource:     entry.Resource,
		Description:  entry.Description,
		IPAddress:    entry.IPAddress,
		UserAgent:    entry.UserAgent,
		RequestID:    entry.RequestID,
		Changes:      entry.Changes,
		Success:      entry.Success,
		ErrorMessage: entry.ErrorMsg,
	}

	// Parse optional UUID fields from string
	if entry.UserID != "" {
		if uid, err := uuid.Parse(entry.UserID); err == nil {
			record.UserID = &uid
		}
	}
	if entry.ResourceID != "" {
		if rid, err := uuid.Parse(entry.ResourceID); err == nil {
			record.ResourceID = &rid
		}
	}

	if err := s.repo.Create(ctx, record); err != nil {
		// Audit failures must never break the main flow — log and continue.
		s.log.Error("failed to write audit log", logger.Error(err))
		return nil
	}
	return nil
}

func (s *auditService) List(ctx context.Context, filter port.AuditFilter, page port.Page) (*port.PageResult[model.AuditLog], error) {
	return s.repo.List(ctx, filter, page)
}
