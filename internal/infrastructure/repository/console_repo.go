package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

type consoleRepo struct {
	db *gorm.DB
}

// NewConsoleRepo creates a GORM-backed ConsoleSessionRepository.
func NewConsoleRepo(db *gorm.DB) port.ConsoleSessionRepository {
	return &consoleRepo{db: db}
}

func (r *consoleRepo) Create(ctx context.Context, s *model.ConsoleSession) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *consoleRepo) GetByToken(ctx context.Context, token string) (*model.ConsoleSession, error) {
	var s model.ConsoleSession
	err := r.db.WithContext(ctx).
		Where("session_token = ?", token).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, port.ErrNotFound
	}
	return &s, err
}

func (r *consoleRepo) GetByID(ctx context.Context, id string) (*model.ConsoleSession, error) {
	var s model.ConsoleSession
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, port.ErrNotFound
	}
	return &s, err
}

func (r *consoleRepo) UpdateStatus(ctx context.Context, id string, status model.ConsoleSessionStatus) error {
	return r.db.WithContext(ctx).
		Model(&model.ConsoleSession{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ExpireOld marks all active sessions whose expires_at is in the past as expired.
func (r *consoleRepo) ExpireOld(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&model.ConsoleSession{}).
		Where("status = ? AND expires_at < ?", model.ConsoleSessionStatusActive, time.Now().UTC()).
		Update("status", model.ConsoleSessionStatusExpired)
	return result.RowsAffected, result.Error
}
