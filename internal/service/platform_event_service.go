package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/internal/notification"
	"github.com/vmOrbit/backend/pkg/logger"
)

// platformEventService implements port.PlatformEventService.
type platformEventService struct {
	repo       port.PlatformEventRepository
	eventBus   messaging.EventBus
	dispatcher *notification.Dispatcher
	log        logger.Logger
}

// NewPlatformEventService creates a new platform event service.
func NewPlatformEventService(
	repo port.PlatformEventRepository,
	eventBus messaging.EventBus,
	dispatcher *notification.Dispatcher,
	log logger.Logger,
) port.PlatformEventService {
	return &platformEventService{
		repo:       repo,
		eventBus:   eventBus,
		dispatcher: dispatcher,
		log:        log,
	}
}

// Dispatch persists a platform event, publishes it to the WebSocket event bus,
// and triggers notification delivery. It is fire-and-forget safe — errors are
// logged but never returned to callers.
func (s *platformEventService) Dispatch(ctx context.Context, req port.EventDispatchRequest) error {
	// Resolve severity
	severity := req.Severity
	if severity == "" {
		severity = model.SeverityForEventType(req.EventType)
	}

	event := &model.PlatformEvent{
		ID:           uuid.New(),
		CreatedAt:    time.Now().UTC(),
		EventType:    req.EventType,
		Severity:     severity,
		Provider:     req.Provider,
		ResourceType: req.ResourceType,
		Message:      req.Message,
		Metadata:     req.Metadata,
	}

	if req.ResourceID != "" {
		if id, err := uuid.Parse(req.ResourceID); err == nil {
			event.ResourceID = &id
		}
	}
	if req.HypervisorID != "" {
		if id, err := uuid.Parse(req.HypervisorID); err == nil {
			event.HypervisorID = &id
		}
	}

	// Persist
	if err := s.repo.Create(ctx, event); err != nil {
		s.log.Error("failed to persist platform event",
			logger.String("event_type", string(req.EventType)),
			logger.Error(err))
		// Non-fatal — continue with bus publish and notifications
	}

	// Publish to WebSocket event bus
	s.eventBus.Publish(ctx, messaging.Event{
		Type: messaging.EventPlatformEvent,
		Payload: map[string]interface{}{
			"id":            event.ID.String(),
			"event_type":    string(event.EventType),
			"severity":      string(event.Severity),
			"provider":      event.Provider,
			"resource_type": event.ResourceType,
			"message":       event.Message,
			"created_at":    event.CreatedAt.UTC().Format(time.RFC3339),
		},
	})

	// Trigger notification delivery (async, non-blocking)
	s.dispatcher.Dispatch(ctx, event)

	return nil
}

func (s *platformEventService) List(ctx context.Context, filter port.PlatformEventFilter, page port.Page) (*port.PageResult[model.PlatformEvent], error) {
	return s.repo.List(ctx, filter, page)
}

func (s *platformEventService) GetByID(ctx context.Context, id string) (*model.PlatformEvent, error) {
	return s.repo.GetByID(ctx, id)
}
