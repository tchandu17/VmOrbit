package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/notification"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ─────────────────────────────────────────────────────────────────────────────
// Notification Channel Service
// ─────────────────────────────────────────────────────────────────────────────

type notificationChannelService struct {
	repo port.NotificationChannelRepository
	log  logger.Logger
}

// NewNotificationChannelService creates a new notification channel service.
func NewNotificationChannelService(
	repo port.NotificationChannelRepository,
	log logger.Logger,
) port.NotificationChannelService {
	return &notificationChannelService{repo: repo, log: log}
}

func (s *notificationChannelService) Create(ctx context.Context, req port.CreateNotificationChannelRequest) (*model.NotificationChannel, error) {
	ch := &model.NotificationChannel{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Enabled:     req.Enabled,
		Config:      req.Config,
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, fmt.Errorf("create notification channel: %w", err)
	}
	return ch, nil
}

func (s *notificationChannelService) GetByID(ctx context.Context, id string) (*model.NotificationChannel, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *notificationChannelService) Update(ctx context.Context, id string, req port.UpdateNotificationChannelRequest) (*model.NotificationChannel, error) {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	if req.Name != nil {
		ch.Name = *req.Name
	}
	if req.Description != nil {
		ch.Description = *req.Description
	}
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	if req.Config != nil {
		ch.Config = req.Config
	}

	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, fmt.Errorf("update notification channel: %w", err)
	}
	return ch, nil
}

func (s *notificationChannelService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *notificationChannelService) List(ctx context.Context) ([]model.NotificationChannel, error) {
	return s.repo.List(ctx)
}

// Test sends a test notification to the channel to verify configuration.
func (s *notificationChannelService) Test(ctx context.Context, id string) error {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("channel not found: %w", err)
	}

	sender, err := notification.NewSender(ch)
	if err != nil {
		return fmt.Errorf("unsupported channel type: %w", err)
	}

	testEvent := &model.PlatformEvent{
		ID:        uuid.New(),
		EventType: model.PlatformEventProviderConnected,
		Severity:  model.PlatformEventSeverityInfo,
		Message:   "This is a test notification from VMOrbit. Your channel is configured correctly.",
		Metadata:  model.JSONMap{"test": true},
	}

	return sender.Send(ctx, ch, testEvent)
}

// ─────────────────────────────────────────────────────────────────────────────
// Notification Rule Service
// ─────────────────────────────────────────────────────────────────────────────

type notificationRuleService struct {
	repo    port.NotificationRuleRepository
	chanRepo port.NotificationChannelRepository
	log     logger.Logger
}

// NewNotificationRuleService creates a new notification rule service.
func NewNotificationRuleService(
	repo port.NotificationRuleRepository,
	chanRepo port.NotificationChannelRepository,
	log logger.Logger,
) port.NotificationRuleService {
	return &notificationRuleService{repo: repo, chanRepo: chanRepo, log: log}
}

func (s *notificationRuleService) Create(ctx context.Context, req port.CreateNotificationRuleRequest) (*model.NotificationRule, error) {
	// Validate channel exists
	chanID, err := uuid.Parse(req.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}
	if _, err := s.chanRepo.GetByID(ctx, req.ChannelID); err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	rule := &model.NotificationRule{
		Name:            req.Name,
		Description:     req.Description,
		ChannelID:       chanID,
		EventTypes:      req.EventTypes,
		Severities:      req.Severities,
		Providers:       req.Providers,
		ThrottleSeconds: req.ThrottleSeconds,
		Enabled:         req.Enabled,
	}
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("create notification rule: %w", err)
	}
	return rule, nil
}

func (s *notificationRuleService) GetByID(ctx context.Context, id string) (*model.NotificationRule, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *notificationRuleService) Update(ctx context.Context, id string, req port.UpdateNotificationRuleRequest) (*model.NotificationRule, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.ChannelID != nil {
		chanID, err := uuid.Parse(*req.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("invalid channel_id: %w", err)
		}
		if _, err := s.chanRepo.GetByID(ctx, *req.ChannelID); err != nil {
			return nil, fmt.Errorf("channel not found: %w", err)
		}
		rule.ChannelID = chanID
	}
	if req.EventTypes != nil {
		rule.EventTypes = req.EventTypes
	}
	if req.Severities != nil {
		rule.Severities = req.Severities
	}
	if req.Providers != nil {
		rule.Providers = req.Providers
	}
	if req.ThrottleSeconds != nil {
		rule.ThrottleSeconds = *req.ThrottleSeconds
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("update notification rule: %w", err)
	}
	return rule, nil
}

func (s *notificationRuleService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *notificationRuleService) List(ctx context.Context) ([]model.NotificationRule, error) {
	return s.repo.List(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// Notification History Service
// ─────────────────────────────────────────────────────────────────────────────

type notificationHistoryService struct {
	repo port.NotificationHistoryRepository
	log  logger.Logger
}

// NewNotificationHistoryService creates a new notification history service.
func NewNotificationHistoryService(
	repo port.NotificationHistoryRepository,
	log logger.Logger,
) port.NotificationHistoryService {
	return &notificationHistoryService{repo: repo, log: log}
}

func (s *notificationHistoryService) List(ctx context.Context, filter port.NotificationHistoryFilter, page port.Page) (*port.PageResult[model.NotificationHistory], error) {
	return s.repo.List(ctx, filter, page)
}
