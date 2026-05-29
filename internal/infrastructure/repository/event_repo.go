package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
)

// EventRepo is the GORM-backed WebSocketEvent repository.
// Events are append-only; the only mutations are MarkDelivered stamps.
type EventRepo struct{ db *gorm.DB }

// NewEventRepo creates a new EventRepo.
func NewEventRepo(db *gorm.DB) *EventRepo { return &EventRepo{db: db} }

// Create persists a new WebSocket event.
func (r *EventRepo) Create(ctx context.Context, event *model.WebSocketEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// ListSince returns events for a room created strictly after `since`,
// ordered oldest-first, capped at `limit` rows.
// Used by clients to catch up after a reconnect.
func (r *EventRepo) ListSince(ctx context.Context, room string, since time.Time, limit int) ([]model.WebSocketEvent, error) {
	var events []model.WebSocketEvent
	err := r.db.WithContext(ctx).
		Where("room = ? AND created_at > ?", room, since).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// MarkDelivered stamps delivered_at = NOW() on the given event IDs.
func (r *EventRepo) MarkDelivered(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.WebSocketEvent{}).
		Where("id IN ?", ids).
		Update("delivered_at", time.Now().UTC()).Error
}
