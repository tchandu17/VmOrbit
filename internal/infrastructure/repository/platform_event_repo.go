package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// PlatformEventRepo is the GORM-backed platform event repository.
// Events are append-only — Create is the only write operation.
type PlatformEventRepo struct{ db *gorm.DB }

// NewPlatformEventRepo creates a new PlatformEventRepo.
func NewPlatformEventRepo(db *gorm.DB) *PlatformEventRepo {
	return &PlatformEventRepo{db: db}
}

func (r *PlatformEventRepo) Create(ctx context.Context, event *model.PlatformEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *PlatformEventRepo) GetByID(ctx context.Context, id string) (*model.PlatformEvent, error) {
	var event model.PlatformEvent
	if err := r.db.WithContext(ctx).First(&event, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *PlatformEventRepo) List(ctx context.Context, filter port.PlatformEventFilter, page port.Page) (*port.PageResult[model.PlatformEvent], error) {
	var items []model.PlatformEvent
	var total int64

	q := r.db.WithContext(ctx).Model(&model.PlatformEvent{})

	if filter.EventType != "" {
		q = q.Where("event_type = ?", filter.EventType)
	}
	if filter.Severity != "" {
		q = q.Where("severity = ?", filter.Severity)
	}
	if filter.Provider != "" {
		q = q.Where("provider = ?", filter.Provider)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceID != nil {
		q = q.Where("resource_id = ?", *filter.ResourceID)
	}
	if filter.HypervisorID != nil {
		q = q.Where("hypervisor_id = ?", *filter.HypervisorID)
	}
	if filter.Since != nil {
		q = q.Where("created_at >= ?", *filter.Since)
	}
	if filter.Until != nil {
		q = q.Where("created_at <= ?", *filter.Until)
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		q = q.Where("message ILIKE ?", like)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("created_at DESC").Offset(offset).Limit(page.Size).Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.PlatformEvent]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

// Ensure interface compliance.
var _ port.PlatformEventRepository = (*PlatformEventRepo)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// Notification Channel Repo
// ─────────────────────────────────────────────────────────────────────────────

// NotificationChannelRepo is the GORM-backed notification channel repository.
type NotificationChannelRepo struct{ db *gorm.DB }

// NewNotificationChannelRepo creates a new NotificationChannelRepo.
func NewNotificationChannelRepo(db *gorm.DB) *NotificationChannelRepo {
	return &NotificationChannelRepo{db: db}
}

func (r *NotificationChannelRepo) Create(ctx context.Context, ch *model.NotificationChannel) error {
	return r.db.WithContext(ctx).Create(ch).Error
}

func (r *NotificationChannelRepo) GetByID(ctx context.Context, id string) (*model.NotificationChannel, error) {
	var ch model.NotificationChannel
	if err := r.db.WithContext(ctx).First(&ch, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *NotificationChannelRepo) Update(ctx context.Context, ch *model.NotificationChannel) error {
	return r.db.WithContext(ctx).Save(ch).Error
}

func (r *NotificationChannelRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.NotificationChannel{}, "id = ?", id).Error
}

func (r *NotificationChannelRepo) List(ctx context.Context) ([]model.NotificationChannel, error) {
	var items []model.NotificationChannel
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&items).Error
	return items, err
}

// Ensure interface compliance.
var _ port.NotificationChannelRepository = (*NotificationChannelRepo)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// Notification Rule Repo
// ─────────────────────────────────────────────────────────────────────────────

// NotificationRuleRepo is the GORM-backed notification rule repository.
type NotificationRuleRepo struct{ db *gorm.DB }

// NewNotificationRuleRepo creates a new NotificationRuleRepo.
func NewNotificationRuleRepo(db *gorm.DB) *NotificationRuleRepo {
	return &NotificationRuleRepo{db: db}
}

func (r *NotificationRuleRepo) Create(ctx context.Context, rule *model.NotificationRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *NotificationRuleRepo) GetByID(ctx context.Context, id string) (*model.NotificationRule, error) {
	var rule model.NotificationRule
	if err := r.db.WithContext(ctx).Preload("Channel").First(&rule, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *NotificationRuleRepo) Update(ctx context.Context, rule *model.NotificationRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *NotificationRuleRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.NotificationRule{}, "id = ?", id).Error
}

func (r *NotificationRuleRepo) List(ctx context.Context) ([]model.NotificationRule, error) {
	var items []model.NotificationRule
	err := r.db.WithContext(ctx).Preload("Channel").Order("created_at DESC").Find(&items).Error
	return items, err
}

func (r *NotificationRuleRepo) ListEnabled(ctx context.Context) ([]model.NotificationRule, error) {
	var items []model.NotificationRule
	err := r.db.WithContext(ctx).
		Preload("Channel").
		Where("enabled = true").
		Order("created_at ASC").
		Find(&items).Error
	return items, err
}

func (r *NotificationRuleRepo) UpdateLastTriggered(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&model.NotificationRule{}).
		Where("id = ?", id).
		Update("last_triggered_at", "NOW()").Error
}

// Ensure interface compliance.
var _ port.NotificationRuleRepository = (*NotificationRuleRepo)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// Notification History Repo
// ─────────────────────────────────────────────────────────────────────────────

// NotificationHistoryRepo is the GORM-backed notification history repository.
type NotificationHistoryRepo struct{ db *gorm.DB }

// NewNotificationHistoryRepo creates a new NotificationHistoryRepo.
func NewNotificationHistoryRepo(db *gorm.DB) *NotificationHistoryRepo {
	return &NotificationHistoryRepo{db: db}
}

func (r *NotificationHistoryRepo) Create(ctx context.Context, h *model.NotificationHistory) error {
	return r.db.WithContext(ctx).Create(h).Error
}

func (r *NotificationHistoryRepo) List(ctx context.Context, filter port.NotificationHistoryFilter, page port.Page) (*port.PageResult[model.NotificationHistory], error) {
	var items []model.NotificationHistory
	var total int64

	q := r.db.WithContext(ctx).Model(&model.NotificationHistory{}).
		Preload("Rule").
		Preload("Channel").
		Preload("Event")

	if filter.RuleID != nil {
		q = q.Where("rule_id = ?", *filter.RuleID)
	}
	if filter.ChannelID != nil {
		q = q.Where("channel_id = ?", *filter.ChannelID)
	}
	if filter.EventID != nil {
		q = q.Where("event_id = ?", *filter.EventID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Since != nil {
		q = q.Where("created_at >= ?", *filter.Since)
	}
	if filter.Until != nil {
		q = q.Where("created_at <= ?", *filter.Until)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("created_at DESC").Offset(offset).Limit(page.Size).Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.NotificationHistory]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

// Ensure interface compliance.
var _ port.NotificationHistoryRepository = (*NotificationHistoryRepo)(nil)

// suppress unused import
var _ = uuid.Nil
